package skiplist

import (
	"sync"
	"unsafe"
)

type INode[K any, V any] interface {
	Key() K
	Value() V
}

// Node คือโหนดแต่ละตัวใน skiplist
type node[K any, V any] struct {
	key      K
	value    V
	backward *node[K, V]   // ตัวชี้ไปยังโหนดก่อนหน้า (เฉพาะชั้น 0)
	forward  []*node[K, V] // สไลซ์ของตัวชี้ไปยังโหนดถัดไปในแต่ละชั้น
	span     []int         // span บอกจำนวนโหนดที่ข้ามไปในแต่ละชั้น
}

func (n *node[K, V]) Key() K {
	return n.key
}

func (n *node[K, V]) Value() V {
	return n.value
}

// reset clears the node's data so it can be safely reused by an allocator.
// It clears pointers to prevent memory leaks and resets slices while retaining
// their underlying capacity for performance.
// reset เคลียร์ข้อมูลในโหนดเพื่อให้ allocator นำกลับมาใช้ใหม่ได้อย่างปลอดภัย
// โดยจะล้างค่า pointer เพื่อป้องกัน memory leak และรีเซ็ต slice แต่ยังคง
// backing array ไว้เพื่อประสิทธิภาพ
func (n *node[K, V]) reset() {
	var zeroK K
	var zeroV V
	n.key, n.value, n.backward = zeroK, zeroV, nil
	clear(n.span)
	clear(n.forward)
}

// --- Node Allocator Abstraction ---

// nodeAllocator defines the interface for memory allocation strategies for nodes.
// This allows swapping between sync.Pool, memory arenas, or other strategies.
// nodeAllocator คือ interface สำหรับกลยุทธ์การจัดสรรหน่วยความจำสำหรับโหนด
// ทำให้สามารถสลับระหว่าง sync.Pool, memory arena, หรือกลยุทธ์อื่นๆ ได้
type nodeAllocator[K any, V any] interface {
	Get() *node[K, V]
	Put(*node[K, V])
	Reset()
}

// --- sync.Pool Implementation ---

// poolAllocator implements nodeAllocator using a sync.Pool.
type poolAllocator[K any, V any] struct {
	pool sync.Pool
}

func newPoolAllocator[K any, V any]() *poolAllocator[K, V] {
	return &poolAllocator[K, V]{
		pool: sync.Pool{
			New: func() any { return &node[K, V]{} },
		},
	}
}

func (p *poolAllocator[K, V]) Get() *node[K, V] {
	return p.pool.Get().(*node[K, V])
}

func (p *poolAllocator[K, V]) Put(n *node[K, V]) {
	// Reset the node to clear its contents before returning it to the pool.
	n.reset()
	p.pool.Put(n)
}

func (p *poolAllocator[K, V]) Reset() {
	// For sync.Pool, we can't truly "reset" it, but we can create a new one
	// to let the old one be garbage collected. This is handled in SkipList.Clear().
	// The pool itself is replaced.
}

// --- Arena Implementation ---

// arenaAllocator implements nodeAllocator using a memory arena.
type arenaAllocator[K any, V any] struct {
	arena *Arena
}

func newArenaAllocator[K any, V any](initialSize int, opts ...ArenaOption) *arenaAllocator[K, V] {
	return &arenaAllocator[K, V]{
		arena: NewArena(initialSize, opts...),
	}
}

// Get allocates a new node from the arena.
// It panics if the arena is out of memory.
func (a *arenaAllocator[K, V]) Get() *node[K, V] {
	nodeSize := unsafe.Sizeof(node[K, V]{})
	nodeAlign := unsafe.Alignof(node[K, V]{})
	ptr := a.arena.Alloc(nodeSize, nodeAlign)
	if ptr == nil {
		panic("skiplist (arena): out of memory")
	}
	// The arena returns raw memory that may contain previous allocations' bytes.
	// That means slice headers and pointer fields inside the node struct
	// could be non-zero (and point to unrelated memory). We explicitly
	// zero the node value here to ensure all headers/pointers are valid
	// Go zero-values before the node is used by the skiplist. This avoids
	// subtle panics when iterator/traversal code reads slice headers.
	n := (*node[K, V])(ptr)
	*n = node[K, V]{}
	return n
}

// Put does nothing for an arena allocator, as memory is reclaimed all at once on Reset.
func (a *arenaAllocator[K, V]) Put(n *node[K, V]) {
	// No-op. The node memory will be reused when the entire arena is reset.
}

// Reset reclaims all memory in the arena, making it available for new allocations.
func (a *arenaAllocator[K, V]) Reset() {
	a.arena.Reset()
}
