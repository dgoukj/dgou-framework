// file: pkg/async/queue.go
package async

import (
	"container/heap"
)

// PriorityItem 优先队列项
type PriorityItem struct {
	Task     *Task // 任务
	Priority int   // 优先级（数字越小优先级越高）
	Index    int   // 在堆中的索引
}

// PriorityQueue 优先队列
type PriorityQueue []*PriorityItem

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// 优先级数字小的在前（优先级高）
	return pq[i].Priority < pq[j].Priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].Index = i
	pq[j].Index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*PriorityItem)
	item.Index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.Index = -1 // 标记为已移除
	*pq = old[0 : n-1]
	return item
}

// Update 更新队列项优先级
func (pq *PriorityQueue) Update(item *PriorityItem, priority int) {
	item.Priority = priority
	heap.Fix(pq, item.Index)
}
