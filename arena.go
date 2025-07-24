package skiplist

import (
	"unsafe"
)

// arenaChunk represents one contiguous block of memory. Arenas are composed of a linked list of chunks.
type arenaChunk struct {
	buf    []byte
	offset uintptr
	next   *arenaChunk
}

// The Arena itself is NOT thread-safe; synchronization must be handled by the caller.
// ตัว Arena เองไม่ thread-safe; ผู้เรียกต้องจัดการ synchronization เอง
type Arena struct {
	root         *arenaChunk // The first chunk in the linked list.
	current      *arenaChunk // The chunk we are currently allocating from.
	growthFactor float64     // Factor to grow by (e.g., 2.0 for 100% growth).
	growthBytes  int         // Bytes to grow by (fixed).
	growthThreshold float64    // Proactive growth threshold (0.0 to 1.0).
}

// ArenaOption configures an Arena.
type ArenaOption func(*Arena)

// WithGrowthFactor sets the arena to grow by a factor of the previous chunk's size.
// For example, a factor of 2.0 means each new chunk will be twice as large as the last.
func WithGrowthFactor(factor float64) ArenaOption {
	return func(a *Arena) {
		if factor > 1.0 {
			a.growthFactor = factor
		}
	}
}

// WithGrowthBytes sets the arena to grow by a fixed number of bytes.
func WithGrowthBytes(bytes int) ArenaOption {
	return func(a *Arena) {
		if bytes > 0 {
			a.growthBytes = bytes
		}
	}
}

// WithGrowthThreshold sets a threshold (e.g., 0.9 for 90%) for proactive growth.
// If an allocation would cause the current chunk's usage to exceed this threshold,
// a new chunk is allocated preemptively. This helps avoid leaving small, unusable
// fragments at the end of chunks. A threshold of 0 disables this feature.
func WithGrowthThreshold(threshold float64) ArenaOption {
	return func(a *Arena) {
		if threshold > 0.0 && threshold < 1.0 {
			a.growthThreshold = threshold
		}
	}
}

// NewArena creates a new, potentially growable Arena.
func NewArena(initialSize int, opts ...ArenaOption) *Arena {
	rootChunk := &arenaChunk{buf: make([]byte, initialSize)}
	a := &Arena{
		root:    rootChunk,
		current: rootChunk,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// align คำนวณ offset ที่ถูกจัดเรียง (aligned) แล้ว
// เพื่อให้แน่ใจว่าที่อยู่ของหน่วยความจำที่จัดสรรไปจะหารด้วยค่า alignment ลงตัว
func align(offset, alignment uintptr) uintptr {
	// (offset + alignment - 1) &^ (alignment - 1)
	// เป็นเทคนิค bit manipulation มาตรฐานสำหรับคำนวณ alignment
	return (offset + alignment - 1) &^ (alignment - 1)
}

// grow allocates a new chunk and links it to the current one. It ensures the
// new chunk is large enough to accommodate `requiredSize`.
// grow จัดสรร chunk ใหม่และเชื่อมต่อกับ chunk ปัจจุบัน โดยจะรับประกันว่า
// chunk ใหม่มีขนาดใหญ่เพียงพอสำหรับ `requiredSize`
func (a *Arena) grow(requiredSize uintptr) {
	var newSize int
	lastChunkSize := len(a.current.buf)

	if a.growthBytes > 0 {
		newSize = a.growthBytes
	} else if a.growthFactor > 1.0 {
		newSize = int(float64(lastChunkSize) * a.growthFactor)
	} else {
		// Default growth is 100% (factor of 2.0) if no strategy is specified.
		newSize = lastChunkSize * 2
	}

	// Ensure the new chunk is large enough for the current request.
	if newSize < int(requiredSize) {
		newSize = int(requiredSize)
	}

	// Create and link the new chunk.
	newChunk := &arenaChunk{buf: make([]byte, newSize)}
	a.current.next = newChunk
	a.current = newChunk
}

// Alloc จัดสรรหน่วยความจำตามขนาด (size) และ alignment ที่ต้องการ
// คืนค่า pointer ไปยังหน่วยความจำที่จัดสรร หรือ nil หาก Arena เต็ม.
// This implementation is NOT thread-safe. The caller must provide synchronization.
// การทำงานนี้ไม่ thread-safe ผู้เรียกต้องจัดการ synchronization เอง
func (a *Arena) Alloc(size, alignment uintptr) unsafe.Pointer {
	// Calculate where the allocation would end up in the current chunk.
	alignedOffset := align(a.current.offset, alignment)
	newOffset := alignedOffset + size

	// Determine if we need to grow.
	// Condition 1: The allocation simply doesn't fit.
	doesNotFit := newOffset > uintptr(len(a.current.buf))
	// Condition 2: The allocation fits, but crosses the proactive growth threshold.
	// A threshold of 0 disables this feature.
	crossesThreshold := a.growthThreshold > 0.0 && !doesNotFit && (newOffset > uintptr(float64(len(a.current.buf))*a.growthThreshold))

	if doesNotFit || crossesThreshold {
		a.grow(size)
		// After growing, we must re-calculate offsets for the new chunk.
		alignedOffset = align(a.current.offset, alignment)
		newOffset = alignedOffset + size
	}

	// At this point, the allocation is guaranteed to fit in the current chunk.
	a.current.offset = newOffset
	return unsafe.Pointer(&a.current.buf[alignedOffset])
}

// Reset ทำให้หน่วยความจำทั้งหมดใน Arena พร้อมใช้งานใหม่ (โดยไม่ได้ลบค่าเก่า)
func (a *Arena) Reset() {
	// Reset offsets on all chunks.
	for c := a.root; c != nil; c = c.next {
		c.offset = 0
	}
	// Start allocating from the first chunk again.
	a.current = a.root
}
