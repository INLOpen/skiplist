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
	// chunks holds slices of nodes. Each chunk is a Go-managed slice of
	// concrete `node` values so that the Go GC is aware of pointer fields
	// inside nodes. This avoids storing Go pointers in raw byte buffers
	// (which the GC would not scan) and prevents subtle memory corruption
	// during long-running benchmarks and GC cycles.
	chunks [][]node[K, V]
	// current index within the last chunk
	pos int
	// next growth size (number of nodes to allocate for the next chunk)
	nextChunkSize int
}

func newArenaAllocator[K any, V any](initialSize int, _opts ...ArenaOption) *arenaAllocator[K, V] {
	// Determine how many nodes the initialSize (in bytes) can hold.
	nodeSize := int(unsafe.Sizeof(node[K, V]{}))
	if nodeSize <= 0 {
		nodeSize = 1
	}
	count := initialSize / nodeSize
	if count < 1 {
		count = 1
	}

	a := &arenaAllocator[K, V]{
		chunks:        make([][]node[K, V], 0, 4),
		pos:           0,
		nextChunkSize: count,
	}
	a.grow() // allocate first chunk
	return a
}

// grow allocates a new chunk of nodes and appends it to chunks.
func (a *arenaAllocator[K, V]) grow() {
	size := a.nextChunkSize
	if size < 1 {
		size = 1
	}
	chunk := make([]node[K, V], size)
	a.chunks = append(a.chunks, chunk)
	a.pos = 0
	// By default, double the next chunk size for exponential growth.
	a.nextChunkSize = size * 2
}

func (a *arenaAllocator[K, V]) Get() *node[K, V] {
	// Ensure we have at least one chunk.
	if len(a.chunks) == 0 {
		a.grow()
	}
	// If current chunk is exhausted, grow.
	if a.pos >= len(a.chunks[len(a.chunks)-1]) {
		a.grow()
	}
	// Return pointer to next node in the current chunk.
	last := &a.chunks[len(a.chunks)-1]
	n := &(*last)[a.pos]
	// Zero the node to ensure a valid Go zero-value (clears slice headers/pointers).
	*n = node[K, V]{}
	a.pos++
	return n
}

func (a *arenaAllocator[K, V]) Put(n *node[K, V]) {
	// No-op. Memory will be reclaimed on Reset().
}

func (a *arenaAllocator[K, V]) Reset() {
	// Keep the first chunk but discard others to allow GC of extra memory.
	if len(a.chunks) == 0 {
		return
	}
	first := a.chunks[0]
	a.chunks = a.chunks[:1]
	a.chunks[0] = first
	a.pos = 0
	// reset growth back to initial chunk size (length of first chunk)
	a.nextChunkSize = len(first)
}
