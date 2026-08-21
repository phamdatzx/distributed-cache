package eviction

import (
	"container/list"
	"fmt"
)

type entry struct {
	key   string
	value any
}

// LRU is a capacity-bounded least-recently-used store.
// It is not safe for concurrent use.
type LRU struct {
	cap       int
	list      *list.List
	items     map[string]*list.Element
	onEvicted func(string, any)
}

// NewLRU creates an LRU with the given capacity.
// onEvicted is optional and called when an entry is removed due to capacity.
func NewLRU(cap int, onEvicted func(string, any)) (*LRU, error) {
	if cap <= 0 {
		return nil, fmt.Errorf("eviction: capacity must be positive, got %d", cap)
	}
	return &LRU{
		cap:       cap,
		list:      list.New(),
		items:     make(map[string]*list.Element, cap),
		onEvicted: onEvicted,
	}, nil
}

func (l *LRU) Set(key string, value any) {
	if ele, ok := l.items[key]; ok {
		ele.Value.(*entry).value = value
		l.list.MoveToFront(ele)
		return
	}
	ele := l.list.PushFront(&entry{key: key, value: value})
	l.items[key] = ele
	if l.list.Len() > l.cap {
		l.removeOldest()
	}
}

func (l *LRU) Get(key string) (any, bool) {
	ele, ok := l.items[key]
	if !ok {
		return nil, false
	}
	l.list.MoveToFront(ele)
	return ele.Value.(*entry).value, true
}

func (l *LRU) Peek(key string) (any, bool) {
	ele, ok := l.items[key]
	if !ok {
		return nil, false
	}
	return ele.Value.(*entry).value, true
}

func (l *LRU) Delete(key string) (any, bool) {
	ele, ok := l.items[key]
	if !ok {
		return nil, false
	}
	return l.removeElement(ele), true
}

func (l *LRU) Len() int {
	return l.list.Len()
}

func (l *LRU) Keys() []string {
	keys := make([]string, 0, len(l.items))
	for k := range l.items {
		keys = append(keys, k)
	}
	return keys
}

func (l *LRU) removeOldest() {
	ele := l.list.Back()
	if ele == nil {
		return
	}
	e := ele.Value.(*entry)
	value := l.removeElement(ele)
	if l.onEvicted != nil {
		l.onEvicted(e.key, value)
	}
}

func (l *LRU) removeElement(ele *list.Element) any {
	e := ele.Value.(*entry)
	l.list.Remove(ele)
	delete(l.items, e.key)
	return e.value
}

// Ensure LRU implements Policy.
var _ Policy = (*LRU)(nil)
