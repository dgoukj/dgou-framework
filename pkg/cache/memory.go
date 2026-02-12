package cache

import (
	"container/list"
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	pkgErrors "github.com/pkg/errors"
)

type memoryItem struct {
	value   string
	expire  time.Time
	size    int
	element *list.Element
}

type MemoryCache struct {
	mu       sync.RWMutex
	items    map[string]*memoryItem
	lruList  *list.List
	lruMap   map[string]*list.Element
	maxBytes int64
	used     int64
	ttl      time.Duration
}

type MemoryConfig struct {
	MaxMemoryMB int
	DefaultTTL  time.Duration
}

func NewMemory(cfg MemoryConfig) *MemoryCache {
	return &MemoryCache{
		items:    make(map[string]*memoryItem),
		lruList:  list.New(),
		lruMap:   make(map[string]*list.Element),
		maxBytes: int64(cfg.MaxMemoryMB) * 1024 * 1024,
		ttl:      cfg.DefaultTTL,
	}
}

func (m *MemoryCache) key(k string) string { return k }

// Get 获取缓存值
func (m *MemoryCache) Get(ctx context.Context, key string) (string, error) {
	m.mu.RLock()
	item, ok := m.items[m.key(key)]
	m.mu.RUnlock()
	if !ok {
		return "", pkgErrors.New("key not found")
	}
	if !item.expire.IsZero() && item.expire.Before(time.Now()) {
		m.Delete(ctx, key)
		return "", pkgErrors.New("key expired")
	}
	// 更新 LRU
	m.mu.Lock()
	if item.element != nil {
		m.lruList.MoveToFront(item.element)
	}
	m.mu.Unlock()
	return item.value, nil
}

// Set 设置缓存值
func (m *MemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	var str string
	switch v := value.(type) {
	case string:
		str = v
	default:
		b, _ := json.Marshal(v)
		str = string(b)
	}
	size := len(m.key(key)) + len(str)

	m.mu.Lock()
	defer m.mu.Unlock()

	// 删除旧值
	if old, ok := m.items[m.key(key)]; ok {
		m.used -= int64(old.size)
		if old.element != nil {
			m.lruList.Remove(old.element)
			delete(m.lruMap, m.key(key))
		}
	}
	// 容量淘汰
	for m.maxBytes > 0 && m.used+int64(size) > m.maxBytes {
		if !m.evictLocked() {
			break
		}
	}
	var expireTime time.Time
	if ttl > 0 {
		expireTime = time.Now().Add(ttl)
	} else if m.ttl > 0 {
		expireTime = time.Now().Add(m.ttl)
	}
	elem := m.lruList.PushFront(m.key(key))
	item := &memoryItem{
		value:   str,
		expire:  expireTime,
		size:    size,
		element: elem,
	}
	m.items[m.key(key)] = item
	m.lruMap[m.key(key)] = elem
	m.used += int64(size)
	return nil
}

// Delete 删除缓存
func (m *MemoryCache) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if item, ok := m.items[m.key(key)]; ok {
		m.used -= int64(item.size)
		if item.element != nil {
			m.lruList.Remove(item.element)
			delete(m.lruMap, m.key(key))
		}
		delete(m.items, m.key(key))
	}
	return nil
}

// Exists 检查键是否存在
func (m *MemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	item, ok := m.items[m.key(key)]
	m.mu.RUnlock()
	if !ok {
		return false, nil
	}
	if !item.expire.IsZero() && item.expire.Before(time.Now()) {
		m.Delete(ctx, key)
		return false, nil
	}
	return true, nil
}

// Ping 健康检查
func (m *MemoryCache) Ping(ctx context.Context) error { return nil }

// Close 关闭缓存
func (m *MemoryCache) Close() error { return nil }

// evictLocked 淘汰最久未使用的条目（需在锁内调用）
func (m *MemoryCache) evictLocked() bool {
	if m.lruList.Len() == 0 {
		return false
	}
	elem := m.lruList.Back()
	if elem == nil {
		return false
	}
	key := elem.Value.(string)
	if item, ok := m.items[key]; ok {
		m.used -= int64(item.size)
		delete(m.items, key)
		delete(m.lruMap, key)
	}
	m.lruList.Remove(elem)
	return true
}

// Incr 自增
func (m *MemoryCache) Incr(ctx context.Context, key string, delta int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.items[m.key(key)]
	if !ok {
		// 键不存在，初始化为 delta
		newVal := strconv.FormatInt(delta, 10)
		size := len(m.key(key)) + len(newVal)
		// 容量淘汰
		for m.maxBytes > 0 && m.used+int64(size) > m.maxBytes {
			if !m.evictLocked() {
				break
			}
		}
		elem := m.lruList.PushFront(m.key(key))
		item = &memoryItem{
			value:   newVal,
			expire:  time.Now().Add(m.ttl),
			size:    size,
			element: elem,
		}
		m.items[m.key(key)] = item
		m.lruMap[m.key(key)] = elem
		m.used += int64(size)
		return delta, nil
	}

	// 解析当前值
	cur, err := strconv.ParseInt(item.value, 10, 64)
	if err != nil {
		return 0, pkgErrors.Wrap(err, "current value is not integer")
	}
	newVal := cur + delta
	item.value = strconv.FormatInt(newVal, 10)
	// 更新 LRU
	if item.element != nil {
		m.lruList.MoveToFront(item.element)
	}
	return newVal, nil
}

// Decr 自减
func (m *MemoryCache) Decr(ctx context.Context, key string, delta int64) (int64, error) {
	return m.Incr(ctx, key, -delta)
}

// ------------------- 以下为高级功能，内存缓存仅提供错误实现 -------------------

func (m *MemoryCache) SAdd(ctx context.Context, key string, members ...interface{}) error {
	return pkgErrors.New("SAdd not implemented in memory cache")
}
func (m *MemoryCache) SRem(ctx context.Context, key string, members ...interface{}) error {
	return pkgErrors.New("SRem not implemented in memory cache")
}
func (m *MemoryCache) SMembers(ctx context.Context, key string) ([]string, error) {
	return nil, pkgErrors.New("SMembers not implemented in memory cache")
}
func (m *MemoryCache) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	return false, pkgErrors.New("SIsMember not implemented in memory cache")
}
func (m *MemoryCache) HSet(ctx context.Context, key, field string, value interface{}) error {
	return pkgErrors.New("HSet not implemented in memory cache")
}
func (m *MemoryCache) HGet(ctx context.Context, key, field string) (string, error) {
	return "", pkgErrors.New("HGet not implemented in memory cache")
}
func (m *MemoryCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return nil, pkgErrors.New("HGetAll not implemented in memory cache")
}
func (m *MemoryCache) HDel(ctx context.Context, key string, fields ...string) error {
	return pkgErrors.New("HDel not implemented in memory cache")
}
func (m *MemoryCache) LPush(ctx context.Context, key string, values ...interface{}) error {
	return pkgErrors.New("LPush not implemented in memory cache")
}
func (m *MemoryCache) LPop(ctx context.Context, key string) (string, error) {
	return "", pkgErrors.New("LPop not implemented in memory cache")
}
func (m *MemoryCache) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return nil, pkgErrors.New("LRange not implemented in memory cache")
}
func (m *MemoryCache) Lock(ctx context.Context, key string, ttl time.Duration) (string, error) {
	return "", pkgErrors.New("Lock not implemented in memory cache")
}
func (m *MemoryCache) Unlock(ctx context.Context, key, token string) error {
	return pkgErrors.New("Unlock not implemented in memory cache")
}
