package cache

import (
	"container/list"
	"sync"
)

type entry struct {
	key   string
	value any
}

type LRU struct {
	mu       sync.Mutex
	capacity int
	items    map[string]*list.Element
	order    *list.List
	hits     uint64
	misses   uint64
}

func NewLRU(capacity int) *LRU {
	if capacity <= 0 {
		capacity = 1
	}
	return &LRU{
		capacity: capacity,
		items:    make(map[string]*list.Element, capacity),
		order:    list.New(),
	}
}

func (c *LRU) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		c.misses++
		return nil, false
	}
	c.hits++
	c.order.MoveToFront(elem)
	return elem.Value.(*entry).value, true
}

func (c *LRU) Put(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		elem.Value.(*entry).value = value
		c.order.MoveToFront(elem)
		return
	}

	elem := c.order.PushFront(&entry{key: key, value: value})
	c.items[key] = elem

	if c.order.Len() > c.capacity {
		oldest := c.order.Back()
		if oldest != nil {
			c.order.Remove(oldest)
			delete(c.items, oldest.Value.(*entry).key)
		}
	}
}

func (c *LRU) Stats() map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	return map[string]any{
		"capacity": c.capacity,
		"size":     c.order.Len(),
		"hits":     c.hits,
		"misses":   c.misses,
	}
}
