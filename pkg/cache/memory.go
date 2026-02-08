package cache

import (
	"container/list"
	"context"
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MemoryCache 内存缓存实现
type MemoryCache struct {
	store       sync.Map                 // 存储数据
	lru         *list.List               // LRU链表（可选）
	lruMap      map[string]*list.Element // LRU映射
	config      *CacheConfig             // 配置
	stats       *CacheStats              // 统计信息
	memoryUsage int64                    // 内存使用量
	maxMemory   int64                    // 最大内存限制
	mu          sync.RWMutex             // 读写锁
	stopCleanup chan bool                // 停止清理信号
}

// cacheEntry 缓存条目
type cacheEntry struct {
	key       string    // 键
	value     string    // 值
	expiresAt time.Time // 过期时间
	size      int64     // 大小（字节）
}

// NewMemoryCache 创建内存缓存实例
func NewMemoryCache(config *CacheConfig) (*MemoryCache, error) {
	mc := &MemoryCache{
		config:      config,
		stats:       &CacheStats{},
		maxMemory:   int64(config.MaxMemoryMB) * 1024 * 1024,
		stopCleanup: make(chan bool),
	}

	// 初始化LRU（如果启用）
	if config.MaxMemoryMB > 0 {
		mc.lru = list.New()
		mc.lruMap = make(map[string]*list.Element)
	}

	// 启动清理协程
	go mc.startCleanup()

	logger.Info("Memory cache initialized",
		logger.Int64("max_memory_mb", int64(config.MaxMemoryMB)),
	)

	return mc, nil
}

// startCleanup 启动定期清理
func (mc *MemoryCache) startCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mc.cleanupExpired()
			if mc.maxMemory > 0 {
				mc.evictIfNeeded()
			}
		case <-mc.stopCleanup:
			return
		}
	}
}

// cleanupExpired 清理过期条目
func (mc *MemoryCache) cleanupExpired() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)

	// 遍历查找过期键
	mc.store.Range(func(key, value interface{}) bool {
		entry := value.(*cacheEntry)
		if entry.expiresAt.Before(now) {
			expiredKeys = append(expiredKeys, entry.key)
		}
		return true
	})

	// 删除过期键
	for _, key := range expiredKeys {
		mc.deleteInternal(key)
	}

	if len(expiredKeys) > 0 {
		logger.Debug("Cleaned up expired cache entries",
			logger.Int("count", len(expiredKeys)),
		)
	}
}

// evictIfNeeded 如果需要则淘汰条目
func (mc *MemoryCache) evictIfNeeded() {
	if mc.memoryUsage <= mc.maxMemory {
		return
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	// 使用LRU策略淘汰
	for mc.memoryUsage > mc.maxMemory && mc.lru.Len() > 0 {
		elem := mc.lru.Back()
		if elem == nil {
			break
		}

		entry := elem.Value.(*cacheEntry)
		mc.deleteInternal(entry.key)
		mc.stats.Evictions++
	}
}

// deleteInternal 内部删除方法
func (mc *MemoryCache) deleteInternal(key string) {
	if value, loaded := mc.store.LoadAndDelete(key); loaded {
		entry := value.(*cacheEntry)
		mc.memoryUsage -= entry.size

		// 从LRU中移除
		if mc.lruMap != nil {
			if elem, ok := mc.lruMap[key]; ok {
				mc.lru.Remove(elem)
				delete(mc.lruMap, key)
			}
		}
	}
}

// updateLRU 更新LRU
func (mc *MemoryCache) updateLRU(key string) {
	if mc.lruMap == nil {
		return
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	if elem, ok := mc.lruMap[key]; ok {
		mc.lru.MoveToFront(elem)
	} else {
		elem := mc.lru.PushFront(key)
		mc.lruMap[key] = elem
	}
}

// Get 获取缓存值
func (mc *MemoryCache) Get(ctx context.Context, key string) (string, error) {
	value, ok := mc.store.Load(key)
	if !ok {
		return "", errors.New(errors.CodeResourceNotFound, "Key not found")
	}

	entry := value.(*cacheEntry)

	// 检查是否过期
	if !entry.expiresAt.IsZero() && entry.expiresAt.Before(time.Now()) {
		mc.deleteInternal(key)
		return "", errors.New(errors.CodeResourceNotFound, "Key expired")
	}

	// 更新LRU
	mc.updateLRU(key)

	return entry.value, nil
}

// Set 设置缓存值
func (mc *MemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	var strValue string
	switch v := value.(type) {
	case string:
		strValue = v
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return errors.Wrap(err, errors.CodeInternalError, "Failed to marshal value")
		}
		strValue = string(data)
	}

	if ttl <= 0 {
		ttl = time.Duration(mc.config.DefaultTTL) * time.Second
	}

	entry := &cacheEntry{
		key:       key,
		value:     strValue,
		expiresAt: time.Now().Add(ttl),
		size:      int64(len(key) + len(strValue)),
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	// 检查是否已存在，先删除旧的
	if old, ok := mc.store.Load(key); ok {
		oldEntry := old.(*cacheEntry)
		mc.memoryUsage -= oldEntry.size

		// 从LRU中移除旧的
		if mc.lruMap != nil {
			if elem, ok := mc.lruMap[key]; ok {
				mc.lru.Remove(elem)
			}
		}
	}

	// 存储新条目
	mc.store.Store(key, entry)
	mc.memoryUsage += entry.size

	// 添加到LRU
	if mc.lruMap != nil {
		elem := mc.lru.PushFront(key)
		mc.lruMap[key] = elem
	}

	// 检查内存使用
	if mc.maxMemory > 0 && mc.memoryUsage > mc.maxMemory {
		mc.evictIfNeeded()
	}

	return nil
}

// Delete 删除缓存值
func (mc *MemoryCache) Delete(ctx context.Context, key string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.deleteInternal(key)
	return nil
}

// Exists 检查键是否存在
func (mc *MemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	value, ok := mc.store.Load(key)
	if !ok {
		return false, nil
	}

	entry := value.(*cacheEntry)
	if !entry.expiresAt.IsZero() && entry.expiresAt.Before(time.Now()) {
		mc.deleteInternal(key)
		return false, nil
	}

	return true, nil
}

// MGet 批量获取
func (mc *MemoryCache) MGet(ctx context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string)

	for _, key := range keys {
		value, err := mc.Get(ctx, key)
		if err == nil {
			result[key] = value
		}
	}

	return result, nil
}

// MSet 批量设置
func (mc *MemoryCache) MSet(ctx context.Context, values map[string]interface{}, ttl time.Duration) error {
	for key, value := range values {
		if err := mc.Set(ctx, key, value, ttl); err != nil {
			return err
		}
	}
	return nil
}

// MDelete 批量删除
func (mc *MemoryCache) MDelete(ctx context.Context, keys []string) error {
	for _, key := range keys {
		_ = mc.Delete(ctx, key)
	}
	return nil
}

// GetOrSet 获取或设置值
func (mc *MemoryCache) GetOrSet(ctx context.Context, key string, fn func() (interface{}, error), ttl time.Duration) (string, error) {
	// 先尝试获取
	value, err := mc.Get(ctx, key)
	if err == nil {
		return value, nil
	}

	// 如果不存在，执行函数获取值
	data, err := fn()
	if err != nil {
		return "", err
	}

	// 设置到缓存
	if err := mc.Set(ctx, key, data, ttl); err != nil {
		return "", err
	}

	return fmt.Sprintf("%v", data), nil
}

// Increment 自增
func (mc *MemoryCache) Increment(ctx context.Context, key string, value int64) (int64, error) {
	current, err := mc.Get(ctx, key)
	if err != nil && !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "Key not found") {
		return 0, err
	}

	var currentNum int64
	if err == nil {
		fmt.Sscanf(current, "%d", &currentNum)
	}

	newValue := currentNum + value
	if err := mc.Set(ctx, key, newValue, 0); err != nil {
		return 0, err
	}

	return newValue, nil
}

// Decrement 自减
func (mc *MemoryCache) Decrement(ctx context.Context, key string, value int64) (int64, error) {
	return mc.Increment(ctx, key, -value)
}

// Expire 设置过期时间
func (mc *MemoryCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	value, err := mc.Get(ctx, key)
	if err != nil {
		return err
	}

	return mc.Set(ctx, key, value, ttl)
}

// TTL 获取剩余过期时间
func (mc *MemoryCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	value, ok := mc.store.Load(key)
	if !ok {
		return -1, errors.New(errors.CodeResourceNotFound, "Key not found")
	}

	entry := value.(*cacheEntry)
	if entry.expiresAt.IsZero() {
		return -1, nil // 永不过期
	}

	now := time.Now()
	if entry.expiresAt.Before(now) {
		mc.deleteInternal(key)
		return -2, errors.New(errors.CodeResourceNotFound, "Key expired") // 已过期
	}

	return entry.expiresAt.Sub(now), nil
}

// SAdd 集合添加成员
func (mc *MemoryCache) SAdd(ctx context.Context, key string, members ...interface{}) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	setKey := fmt.Sprintf("set:%s", key)

	// 获取现有的集合数据
	var setMembers []string
	if value, ok := mc.store.Load(setKey); ok {
		entry := value.(*cacheEntry)
		if err := json.Unmarshal([]byte(entry.value), &setMembers); err != nil {
			setMembers = []string{}
		}
	} else {
		setMembers = []string{}
	}

	// 添加新成员（去重）
	existing := make(map[string]bool)
	for _, member := range setMembers {
		existing[member] = true
	}

	for _, member := range members {
		var memberStr string
		switch v := member.(type) {
		case string:
			memberStr = v
		default:
			data, err := json.Marshal(member)
			if err != nil {
				return errors.Wrap(err, errors.CodeInternalError, "Failed to marshal member")
			}
			memberStr = string(data)
		}

		if !existing[memberStr] {
			setMembers = append(setMembers, memberStr)
			existing[memberStr] = true
		}
	}

	// 序列化并存储
	data, err := json.Marshal(setMembers)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to marshal set members")
	}

	// 计算大小差异
	var oldSize int64
	if oldValue, ok := mc.store.Load(setKey); ok {
		oldEntry := oldValue.(*cacheEntry)
		oldSize = oldEntry.size
	}

	entry := &cacheEntry{
		key:       setKey,
		value:     string(data),
		expiresAt: time.Time{}, // 永不过期，除非显式设置
		size:      int64(len(setKey) + len(data)),
	}

	mc.store.Store(setKey, entry)
	mc.memoryUsage += entry.size - oldSize

	return nil
}

// SRem 集合删除成员
func (mc *MemoryCache) SRem(ctx context.Context, key string, members ...interface{}) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	setKey := fmt.Sprintf("set:%s", key)

	// 获取现有的集合数据
	value, ok := mc.store.Load(setKey)
	if !ok {
		return nil // 集合不存在，无需删除
	}

	entry := value.(*cacheEntry)
	var setMembers []string
	if err := json.Unmarshal([]byte(entry.value), &setMembers); err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to unmarshal set members")
	}

	// 创建要删除的成员映射
	toRemove := make(map[string]bool)
	for _, member := range members {
		var memberStr string
		switch v := member.(type) {
		case string:
			memberStr = v
		default:
			data, err := json.Marshal(member)
			if err != nil {
				return errors.Wrap(err, errors.CodeInternalError, "Failed to marshal member")
			}
			memberStr = string(data)
		}
		toRemove[memberStr] = true
	}

	// 过滤出要保留的成员
	var newMembers []string
	for _, member := range setMembers {
		if !toRemove[member] {
			newMembers = append(newMembers, member)
		}
	}

	// 如果集合为空，删除整个键
	if len(newMembers) == 0 {
		mc.deleteInternal(setKey)
		return nil
	}

	// 序列化并存储更新后的数据
	data, err := json.Marshal(newMembers)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to marshal set members")
	}

	newEntry := &cacheEntry{
		key:       setKey,
		value:     string(data),
		expiresAt: entry.expiresAt, // 保持原有过期时间
		size:      int64(len(setKey) + len(data)),
	}

	mc.store.Store(setKey, newEntry)
	mc.memoryUsage += newEntry.size - entry.size

	return nil
}

// SMembers 获取集合所有成员
func (mc *MemoryCache) SMembers(ctx context.Context, key string) ([]string, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	setKey := fmt.Sprintf("set:%s", key)
	value, ok := mc.store.Load(setKey)
	if !ok {
		return []string{}, nil
	}

	entry := value.(*cacheEntry)

	// 解析集合数据
	var members []string
	if err := json.Unmarshal([]byte(entry.value), &members); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to unmarshal set members")
	}

	return members, nil
}

// SIsMember 判断元素是否是集合成员
func (mc *MemoryCache) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	members, err := mc.SMembers(ctx, key)
	if err != nil {
		return false, err
	}

	var memberStr string
	switch v := member.(type) {
	case string:
		memberStr = v
	default:
		data, err := json.Marshal(member)
		if err != nil {
			return false, errors.Wrap(err, errors.CodeInternalError, "Failed to marshal member")
		}
		memberStr = string(data)
	}

	for _, m := range members {
		if m == memberStr {
			return true, nil
		}
	}

	return false, nil
}

// HSet 设置哈希字段值
func (mc *MemoryCache) HSet(ctx context.Context, key string, field string, value interface{}) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	hashKey := fmt.Sprintf("hash:%s", key)

	// 获取现有的哈希数据
	var hashData map[string]string
	if value, ok := mc.store.Load(hashKey); ok {
		entry := value.(*cacheEntry)
		if err := json.Unmarshal([]byte(entry.value), &hashData); err != nil {
			hashData = make(map[string]string)
		}
	} else {
		hashData = make(map[string]string)
	}

	// 设置字段值
	var strValue string
	switch v := value.(type) {
	case string:
		strValue = v
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return errors.Wrap(err, errors.CodeInternalError, "Failed to marshal value")
		}
		strValue = string(data)
	}

	hashData[field] = strValue

	// 序列化并存储
	data, err := json.Marshal(hashData)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to marshal hash data")
	}

	entry := &cacheEntry{
		key:       hashKey,
		value:     string(data),
		expiresAt: time.Time{}, // 永不过期，除非显式设置
		size:      int64(len(hashKey) + len(data)),
	}

	mc.store.Store(hashKey, entry)
	mc.memoryUsage += entry.size

	return nil
}

// HGet 获取哈希字段值
func (mc *MemoryCache) HGet(ctx context.Context, key string, field string) (string, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	hashKey := fmt.Sprintf("hash:%s", key)
	value, ok := mc.store.Load(hashKey)
	if !ok {
		return "", errors.New(errors.CodeResourceNotFound, "Hash not found")
	}

	entry := value.(*cacheEntry)

	// 解析哈希数据
	var hashData map[string]string
	if err := json.Unmarshal([]byte(entry.value), &hashData); err != nil {
		return "", errors.Wrap(err, errors.CodeInternalError, "Failed to unmarshal hash data")
	}

	fieldValue, ok := hashData[field]
	if !ok {
		return "", errors.New(errors.CodeResourceNotFound, "Field not found")
	}

	return fieldValue, nil
}

// HGetAll 获取哈希所有字段
func (mc *MemoryCache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	hashKey := fmt.Sprintf("hash:%s", key)
	value, ok := mc.store.Load(hashKey)
	if !ok {
		return map[string]string{}, nil
	}

	entry := value.(*cacheEntry)

	// 解析哈希数据
	var hashData map[string]string
	if err := json.Unmarshal([]byte(entry.value), &hashData); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to unmarshal hash data")
	}

	return hashData, nil
}

// HDelete 删除哈希字段
func (mc *MemoryCache) HDelete(ctx context.Context, key string, fields ...string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	hashKey := fmt.Sprintf("hash:%s", key)
	value, ok := mc.store.Load(hashKey)
	if !ok {
		return nil
	}

	entry := value.(*cacheEntry)

	// 解析哈希数据
	var hashData map[string]string
	if err := json.Unmarshal([]byte(entry.value), &hashData); err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to unmarshal hash data")
	}

	// 删除指定字段
	for _, field := range fields {
		delete(hashData, field)
	}

	// 如果哈希为空，删除整个键
	if len(hashData) == 0 {
		mc.deleteInternal(hashKey)
		return nil
	}

	// 序列化并存储更新后的数据
	data, err := json.Marshal(hashData)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to marshal hash data")
	}

	newEntry := &cacheEntry{
		key:       hashKey,
		value:     string(data),
		expiresAt: entry.expiresAt, // 保持原有过期时间
		size:      int64(len(hashKey) + len(data)),
	}

	mc.store.Store(hashKey, newEntry)
	mc.memoryUsage += newEntry.size - entry.size

	return nil
}

// LPush 列表推送
func (mc *MemoryCache) LPush(ctx context.Context, key string, values ...interface{}) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	listKey := fmt.Sprintf("list:%s", key)

	// 获取现有的列表数据
	var listValues []string
	if value, ok := mc.store.Load(listKey); ok {
		entry := value.(*cacheEntry)
		if err := json.Unmarshal([]byte(entry.value), &listValues); err != nil {
			listValues = []string{}
		}
	} else {
		listValues = []string{}
	}

	// 将新值添加到列表头部
	var newValues []string
	for _, val := range values {
		var strValue string
		switch v := val.(type) {
		case string:
			strValue = v
		default:
			data, err := json.Marshal(val)
			if err != nil {
				return errors.Wrap(err, errors.CodeInternalError, "Failed to marshal value")
			}
			strValue = string(data)
		}
		newValues = append(newValues, strValue)
	}

	// 新值在前，旧值在后
	listValues = append(newValues, listValues...)

	// 序列化并存储
	data, err := json.Marshal(listValues)
	if err != nil {
		return errors.Wrap(err, errors.CodeInternalError, "Failed to marshal list values")
	}

	// 计算大小差异
	var oldSize int64
	if oldValue, ok := mc.store.Load(listKey); ok {
		oldEntry := oldValue.(*cacheEntry)
		oldSize = oldEntry.size
	}

	entry := &cacheEntry{
		key:       listKey,
		value:     string(data),
		expiresAt: time.Time{}, // 永不过期，除非显式设置
		size:      int64(len(listKey) + len(data)),
	}

	mc.store.Store(listKey, entry)
	mc.memoryUsage += entry.size - oldSize

	return nil
}

// LPop 列表弹出
func (mc *MemoryCache) LPop(ctx context.Context, key string) (string, error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	listKey := fmt.Sprintf("list:%s", key)

	// 获取现有的列表数据
	value, ok := mc.store.Load(listKey)
	if !ok {
		return "", errors.New(errors.CodeResourceNotFound, "List not found")
	}

	entry := value.(*cacheEntry)
	var listValues []string
	if err := json.Unmarshal([]byte(entry.value), &listValues); err != nil {
		return "", errors.Wrap(err, errors.CodeInternalError, "Failed to unmarshal list values")
	}

	if len(listValues) == 0 {
		return "", errors.New(errors.CodeResourceNotFound, "List is empty")
	}

	// 弹出第一个元素
	popped := listValues[0]
	listValues = listValues[1:]

	// 如果列表为空，删除整个键
	if len(listValues) == 0 {
		mc.deleteInternal(listKey)
		return popped, nil
	}

	// 序列化并存储更新后的数据
	data, err := json.Marshal(listValues)
	if err != nil {
		return "", errors.Wrap(err, errors.CodeInternalError, "Failed to marshal list values")
	}

	newEntry := &cacheEntry{
		key:       listKey,
		value:     string(data),
		expiresAt: entry.expiresAt, // 保持原有过期时间
		size:      int64(len(listKey) + len(data)),
	}

	mc.store.Store(listKey, newEntry)
	mc.memoryUsage += newEntry.size - entry.size

	return popped, nil
}

// LRange 获取列表范围
func (mc *MemoryCache) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	listKey := fmt.Sprintf("list:%s", key)

	// 获取现有的列表数据
	value, ok := mc.store.Load(listKey)
	if !ok {
		return []string{}, nil
	}

	entry := value.(*cacheEntry)
	var listValues []string
	if err := json.Unmarshal([]byte(entry.value), &listValues); err != nil {
		return nil, errors.Wrap(err, errors.CodeInternalError, "Failed to unmarshal list values")
	}

	// 处理负索引
	length := int64(len(listValues))
	if start < 0 {
		start = length + start
	}
	if stop < 0 {
		stop = length + stop
	}

	// 边界检查
	if start < 0 {
		start = 0
	}
	if start >= length {
		return []string{}, nil
	}
	if stop >= length {
		stop = length - 1
	}
	if stop < start {
		return []string{}, nil
	}

	return listValues[start : stop+1], nil
}

// Publish 发布消息
func (mc *MemoryCache) Publish(ctx context.Context, channel string, message interface{}) error {
	// 内存缓存不支持发布订阅，返回错误
	return errors.New(errors.CodeInternalError, "Pub/Sub not supported in memory cache")
}

// Subscribe 订阅消息
func (mc *MemoryCache) Subscribe(ctx context.Context, channel string, handler func(message string)) error {
	// 内存缓存不支持发布订阅，返回错误
	return errors.New(errors.CodeInternalError, "Pub/Sub not supported in memory cache")
}

// Lock 获取分布式锁
func (mc *MemoryCache) Lock(ctx context.Context, key string, ttl time.Duration) (string, error) {
	lockKey := fmt.Sprintf("lock:%s", key)
	token := generateLockToken()

	// 尝试设置锁
	err := mc.Set(ctx, lockKey, token, ttl)
	if err != nil {
		return "", err
	}

	// 检查是否成功获取锁
	existingToken, err := mc.Get(ctx, lockKey)
	if err != nil || existingToken != token {
		return "", errors.New(errors.CodeInternalError, "Failed to acquire lock")
	}

	return token, nil
}

// Unlock 释放分布式锁
func (mc *MemoryCache) Unlock(ctx context.Context, key, token string) error {
	lockKey := fmt.Sprintf("lock:%s", key)

	// 检查令牌是否匹配
	existingToken, err := mc.Get(ctx, lockKey)
	if err != nil {
		return err
	}

	if existingToken != token {
		return errors.New(errors.CodeInternalError, "Token mismatch")
	}

	// 删除锁
	return mc.Delete(ctx, lockKey)
}

// TryLock 尝试获取分布式锁
func (mc *MemoryCache) TryLock(ctx context.Context, key string, ttl time.Duration, waitTime time.Duration) (string, error) {
	start := time.Now()
	for time.Since(start) < waitTime {
		token, err := mc.Lock(ctx, key, ttl)
		if err == nil {
			return token, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return "", errors.New(errors.CodeInternalError, "Timeout acquiring lock")
}

// GetBit 获取位值
func (mc *MemoryCache) GetBit(ctx context.Context, key string, offset int64) (int64, error) {
	// 简化实现：对于内存缓存，位操作不常用
	return 0, errors.New(errors.CodeInternalError, "Bit operations not supported in memory cache")
}

// SetBit 设置位值
func (mc *MemoryCache) SetBit(ctx context.Context, key string, offset int64, value int) error {
	return errors.New(errors.CodeInternalError, "Bit operations not supported in memory cache")
}

// Clear 清空缓存
func (mc *MemoryCache) Clear(ctx context.Context) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// 清空存储
	mc.store = sync.Map{}

	// 清空LRU
	if mc.lru != nil {
		mc.lru.Init()
		mc.lruMap = make(map[string]*list.Element)
	}

	mc.memoryUsage = 0

	return nil
}

// Stats 返回统计信息
func (mc *MemoryCache) Stats() CacheStats {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	stats := *mc.stats
	stats.MemoryUsage = mc.memoryUsage
	return stats
}

// Close 关闭内存缓存
func (mc *MemoryCache) Close() error {
	mc.stopCleanup <- true
	return nil
}

// Ping 实现Ping方法
func (mc *MemoryCache) Ping(ctx context.Context) error {
	// 内存缓存总是可用的
	return nil
}

// MemoryBloomFilter 内存布隆过滤器
type MemoryBloomFilter struct {
	bitset []byte
	size   int64
	mu     sync.RWMutex
}

// NewMemoryBloomFilter 创建内存布隆过滤器
func NewMemoryBloomFilter(size int64, errorRate float64) (*MemoryBloomFilter, error) {
	// 计算需要的位数和哈希函数数量
	// 简化实现，固定参数
	if size <= 0 {
		size = 100000
	}

	// 每个元素8位
	bitSize := size * 8
	bitset := make([]byte, (bitSize+7)/8)

	return &MemoryBloomFilter{
		bitset: bitset,
		size:   bitSize,
	}, nil
}

// Add 添加元素
func (bf *MemoryBloomFilter) Add(ctx context.Context, key string, value string) error {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	hash1 := hash1(value) % bf.size
	hash2 := hash2(value) % bf.size

	bf.setBit(hash1)
	bf.setBit(hash2)

	return nil
}

// Exists 检查元素是否存在
func (bf *MemoryBloomFilter) Exists(ctx context.Context, key string, value string) (bool, error) {
	bf.mu.RLock()
	defer bf.mu.RUnlock()

	hash1 := hash1(value) % bf.size
	hash2 := hash2(value) % bf.size

	return bf.getBit(hash1) && bf.getBit(hash2), nil
}

// Clear 清空布隆过滤器
func (bf *MemoryBloomFilter) Clear(ctx context.Context, key string) error {
	bf.mu.Lock()
	defer bf.mu.Unlock()

	for i := range bf.bitset {
		bf.bitset[i] = 0
	}

	return nil
}

// setBit 设置位
func (bf *MemoryBloomFilter) setBit(pos int64) {
	byteIndex := pos / 8
	bitIndex := pos % 8
	bf.bitset[byteIndex] |= 1 << bitIndex
}

// getBit 获取位
func (bf *MemoryBloomFilter) getBit(pos int64) bool {
	byteIndex := pos / 8
	bitIndex := pos % 8
	return (bf.bitset[byteIndex] & (1 << bitIndex)) != 0
}
