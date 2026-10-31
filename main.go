package main

import (
	"container/heap"
	"container/list"
	"fmt"
	"time"
)

/**
实现一个带过期时间支持的LRUCache

对外提供三个函数接口：构造函数、Put函数、Get函数

构造函数输入一个容量参数指定整个LRUCache的最大容量限制

Put函数输入key、value、ttl，其中ttl是过期时间，如果小于等于0就表示不过期，大于0表示对应key/value存活的秒数，
函数要求将key/value放入LRUCache中且保证是最近访问的，同时还要确保容量不能超限制，如果超过则需要淘汰一个旧的key/value对，
淘汰逻辑是：优先淘汰一个已过期的，不存在已过期的情况下则按LRU策略淘汰最久没有访问的那个。

Get函数输入key返回value，要求根据key查找对应的value，如果存在且未过期则调整key/value为最近访问返回对应value，
如果已过期则删除对应key/value后返回空，不存在则直接返回空。
*/

type Node struct {
	k        string
	v        string
	ttl      int64 // put 时传入的存活秒数；<=0 表示永不过期
	expireAt int64 // Unix 秒，过期时刻；0 表示永不过期
	elem     *list.Element
	pqIndex  int
}

type PriorityQueue []*Node

func (pq PriorityQueue) Len() int { return len(pq) }

// 过期更早的排在前面；永不过期的 (expireAt == 0) 永远靠后
func (pq PriorityQueue) Less(i, j int) bool {
	if pq[i].expireAt == 0 && pq[j].expireAt == 0 {
		return false
	}
	if pq[i].expireAt == 0 {
		return false // i 永不过期，排后面
	}
	if pq[j].expireAt == 0 {
		return true // j 永不过期，i 排前面
	}
	return pq[i].expireAt < pq[j].expireAt
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].pqIndex = i
	pq[j].pqIndex = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := x.(*Node)
	n.pqIndex = len(*pq)
	*pq = append(*pq, n)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.pqIndex = -1
	*pq = old[0 : n-1]
	return item
}

type LRUCache struct {
	capacity int
	m        map[string]*Node
	l        *list.List
	pq       PriorityQueue
}

func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		m:        make(map[string]*Node),
		l:        list.New(),
		pq:       make(PriorityQueue, 0),
	}
}

func nowMs() int64 { return time.Now().UnixNano() / 1e6 }

func (c *LRUCache) isExpired(n *Node) bool {
	if n.expireAt == 0 {
		return false
	}
	return nowMs() >= n.expireAt
}

// pickVictim 返回一个要淘汰的节点：优先过期堆顶，否则 LRU 队尾；没节点可淘汰返回 nil
func (c *LRUCache) pickVictim() *Node {
	if c.pq.Len() > 0 {
		top := c.pq[0]
		if c.isExpired(top) {
			return top
		}
	}
	if tail := c.l.Back(); tail != nil {
		return tail.Value.(*Node)
	}
	return nil
}

// remove 把 node 从 map / list / heap 三处都删掉，O(log n)
func (c *LRUCache) remove(n *Node) {
	if n == nil {
		return
	}
	delete(c.m, n.k)
	if n.elem != nil {
		c.l.Remove(n.elem)
	}
	if n.pqIndex >= 0 && n.pqIndex < c.pq.Len() && c.pq[n.pqIndex] == n {
		heap.Remove(&c.pq, n.pqIndex)
	}
}

func (c *LRUCache) Put(key, value string, ttl int64) {
	expireAt := int64(0)
	if ttl > 0 {
		expireAt = nowMs() + ttl*1000
	}

	// 已存在 → 更新值 / 过期时间，提为最近访问，修复堆
	if old, ok := c.m[key]; ok {
		old.v = value
		old.ttl = ttl
		old.expireAt = expireAt
		c.l.MoveToFront(old.elem)
		heap.Fix(&c.pq, old.pqIndex)
		return
	}

	// 容量满 → 淘汰（循环直到能放下，理论上一轮就够）
	for c.l.Len() >= c.capacity {
		victim := c.pickVictim()
		if victim == nil {
			break
		}
		c.remove(victim)
	}

	n := &Node{k: key, v: value, ttl: ttl, expireAt: expireAt}
	n.elem = c.l.PushFront(n)
	heap.Push(&c.pq, n)
	c.m[key] = n
}

func (c *LRUCache) Get(key string) (string, bool) {
	n, ok := c.m[key]
	if !ok {
		return "", false
	}
	if c.isExpired(n) {
		c.remove(n)
		return "", false
	}
	c.l.MoveToFront(n.elem)
	return n.v, true
}

func main() {
	printGet := func(label string, c *LRUCache, key string) {
		v, ok := c.Get(key)
		fmt.Printf("%-30s key=%q -> ok=%v, v=%q\n", label, key, ok, v)
	}

	fmt.Println("=== Test 1: 基本 Put/Get + 容量淘汰 ===")
	c := NewLRUCache(3)
	c.Put("a", "1", 0)
	c.Put("b", "2", 0)
	c.Put("c", "3", 0)
	printGet("访问 a 后应成为最近", c, "a")
	c.Put("d", "4", 0) // 容量满，应淘汰 LRU 的 b
	printGet("b 应被淘汰(LRU)", c, "b")
	printGet("c 应还在", c, "c")
	printGet("d 新加入应在", c, "d")

	fmt.Println("\n=== Test 2: TTL 过期淘汰 ===")
	c2 := NewLRUCache(3)
	c2.Put("x", "1", 0) // 永不过期
	c2.Put("y", "2", 1) // 1 秒后过期
	c2.Put("z", "3", 2) // 2 秒后过期
	time.Sleep(1100 * time.Millisecond)
	printGet("y 1s 后应过期", c2, "y")
	printGet("x 永不过期应还在", c2, "x")

	fmt.Println("\n=== Test 3: 容量满时优先淘汰过期的，不是 LRU ===")
	c3 := NewLRUCache(3)
	c3.Put("a", "1", 1) // 会过期
	c3.Put("b", "2", 0) // 永不过期、LRU 队尾
	c3.Put("c", "3", 0)
	time.Sleep(1100 * time.Millisecond)
	c3.Put("d", "4", 0) // 应淘汰已过期的 a，而不是 LRU 的 b
	printGet("a 应过期被淘汰", c3, "a")
	printGet("b(LRU队尾)应被保留", c3, "b")
	printGet("c 应还在", c3, "c")
	printGet("d 新加入应在", c3, "d")

	fmt.Println("\n=== Test 4: Put 已存在的 key 刷新值和过期时间 ===")
	c4 := NewLRUCache(2)
	c4.Put("k", "old", 1)
	time.Sleep(600 * time.Millisecond)
	c4.Put("k", "new", 10)             // 刷新过期时间
	time.Sleep(600 * time.Millisecond) // 距第一次 Put 已 1.2s，但第二次 Put 重置了 ttl
	printGet("k 应还在且值为 new", c4, "k")

	fmt.Println("\n=== Test 5: 容量 1 ===")
	c5 := NewLRUCache(1)
	c5.Put("a", "1", 0)
	c5.Put("b", "2", 0)
	printGet("a 应被淘汰", c5, "a")
	printGet("b 应还在", c5, "b")

	fmt.Println("\nAll tests done.")
}
