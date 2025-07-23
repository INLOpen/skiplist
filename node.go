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
	// เคลียร์ค่าในโหนดก่อนคืนเข้า pool เพื่อป้องกัน memory leak
	var zeroK K
	var zeroV V
	n.key = zeroK
	n.value = zeroV
	n.backward = nil
	// เคลียร์ slice ทั้งสองเพื่อล้างข้อมูลเก่า แต่ยังคงเก็บ backing array ไว้เพื่อนำกลับมาใช้ใหม่
	// ซึ่งเป็นหัวใจสำคัญของการทำ pooling optimization
	clear(n.span)
	clear(n.forward)
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

func newArenaAllocator[K any, V any](sizeInBytes int) *arenaAllocator[K, V] {
	return &arenaAllocator[K, V]{
		arena: NewArena(sizeInBytes),
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
	// The memory from the arena is not zeroed, so we get a pointer to it
	// and must initialize it properly. The `forward` slice will be nil.
	return (*node[K, V])(ptr)
}

// Put does nothing for an arena allocator, as memory is reclaimed all at once on Reset.
func (a *arenaAllocator[K, V]) Put(n *node[K, V]) {
	// No-op. The node memory will be reused when the entire arena is reset.
}

// Reset reclaims all memory in the arena, making it available for new allocations.
func (a *arenaAllocator[K, V]) Reset() {
	a.arena.Reset()
}
