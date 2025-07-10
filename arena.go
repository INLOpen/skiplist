package skiplist

import (
	"sync/atomic"
	"unsafe"
)

// Arena provides a way to allocate memory for nodes from a large, pre-allocated block.
// This reduces GC overhead and can improve performance by improving data locality.
// It is not a general-purpose allocator; memory is only reclaimed when the entire arena is reset.
// Arena คือ Allocator ที่จัดการหน่วยความจำก้อนใหญ่ที่จองไว้ล่วงหน้า
// ช่วยลดภาระของ GC และเพิ่มประสิทธิภาพจาก Data Locality
// หน่วยความจำจะถูกนำกลับมาใช้ใหม่ได้ก็ต่อเมื่อมีการ Reset ทั้ง Arena เท่านั้น
type Arena struct {
	buf    []byte
	offset uintptr // This will be managed atomically.
}

// NewArena สร้าง Arena ใหม่ด้วยขนาดที่กำหนด (เป็น byte)
func NewArena(size int) *Arena {
	return &Arena{
		buf: make([]byte, size),
	}
}

// align คำนวณ offset ที่ถูกจัดเรียง (aligned) แล้ว
// เพื่อให้แน่ใจว่าที่อยู่ของหน่วยความจำที่จัดสรรไปจะหารด้วยค่า alignment ลงตัว
func align(offset, alignment uintptr) uintptr {
	// (offset + alignment - 1) &^ (alignment - 1)
	// เป็นเทคนิค bit manipulation มาตรฐานสำหรับคำนวณ alignment
	return (offset + alignment - 1) &^ (alignment - 1)
}

// Alloc จัดสรรหน่วยความจำตามขนาด (size) และ alignment ที่ต้องการ
// คืนค่า pointer ไปยังหน่วยความจำที่จัดสรร หรือ nil หาก Arena เต็ม
// This implementation is lock-free and uses a Compare-And-Swap (CAS) loop.
func (a *Arena) Alloc(size, alignment uintptr) unsafe.Pointer {
	for {
		// 1. Atomically load the current offset.
		currentOffset := atomic.LoadUintptr(&a.offset)

		// 2. Calculate the new aligned offset.
		alignedOffset := align(currentOffset, alignment)
		newOffset := alignedOffset + size

		// 3. Check if the arena has enough space.
		if newOffset > uintptr(len(a.buf)) {
			return nil // Arena is full.
		}

		// 4. Attempt to atomically update the offset from the value we loaded.
		// If it succeeds, we have successfully reserved the memory block.
		if atomic.CompareAndSwapUintptr(&a.offset, currentOffset, newOffset) {
			return unsafe.Pointer(&a.buf[alignedOffset])
		}
		// If it fails, another goroutine won the race. Loop and try again.
	}
}

// Reset ทำให้หน่วยความจำทั้งหมดใน Arena พร้อมใช้งานใหม่ (โดยไม่ได้ลบค่าเก่า)
func (a *Arena) Reset() {
	atomic.StoreUintptr(&a.offset, 0)
}
