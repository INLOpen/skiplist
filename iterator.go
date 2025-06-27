package skiplist

// Iterator provides a way to iterate over the elements of a SkipList.
// An iterator must be created with the NewIterator method.
// Iterator คือโครงสร้างที่ใช้สำหรับวนลูปผ่านรายการใน Skiplist
type Iterator[K any, V any] struct {
	sl      *SkipList[K, V] // อ้างอิงถึง Skiplist ที่กำลังวนลูป
	current *Node[K, V]     // โหนดปัจจุบันที่ Iterator ชี้อยู่
	unsafe  bool            // ถ้าเป็น true, จะไม่ทำการ lock/unlock (ใช้สำหรับ RangeWithIterator)
}

// NewIterator creates a new iterator positioned at the first element of the skiplist.
// NewIterator สร้างและคืนค่า Iterator ใหม่ที่ชี้ไปยังรายการแรกใน Skiplist
func (sl *SkipList[K, V]) NewIterator() *Iterator[K, V] {
	// สร้าง iterator แบบ thread-safe ปกติ
	return &Iterator[K, V]{
		sl:      sl,
		current: sl.header.forward[0], // เริ่มต้นที่โหนดแรก
		unsafe:  false,
	}
}

// Valid returns true if the iterator is positioned at a valid element.
// It returns false if the iterator is exhausted.
// Valid ตรวจสอบว่า Iterator ชี้ไปยังรายการที่ถูกต้องหรือไม่ (ไม่ใช่ nil)
func (it *Iterator[K, V]) Valid() bool {
	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}
	return it.current != nil
}

// Key returns the key of the element at the current iterator position.
// It returns the zero value for K if the iterator is not valid.
// Key คืนค่า key ของรายการปัจจุบันที่ Iterator ชี้อยู่
// หาก Iterator ไม่ Valid จะคืนค่า zero value ของ K
func (it *Iterator[K, V]) Key() K {
	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}
	if it.current == nil {
		var zeroK K
		return zeroK
	}
	return it.current.Key
}

// Value returns the value of the element at the current iterator position.
// It returns the zero value for V if the iterator is not valid.
// Value คืนค่า value ของรายการปัจจุบันที่ Iterator ชี้อยู่
// หาก Iterator ไม่ Valid จะคืนค่า zero value ของ V
func (it *Iterator[K, V]) Value() V {
	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}
	if it.current == nil {
		var zeroV V
		return zeroV
	}
	return it.current.Value
}

// Next moves the iterator to the next element in the skiplist.
// If the iterator is not valid or there are no more elements, the iterator becomes invalid.
// Next เลื่อน Iterator ไปยังรายการถัดไป
// หาก Iterator ไม่ Valid หรือไม่มีรายการถัดไป จะทำให้ Iterator ไม่ Valid
func (it *Iterator[K, V]) Next() {
	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}
	if it.current != nil {
		it.current = it.current.forward[0]
	}
}

// Rewind moves the iterator back to the first element of the skiplist.
// Rewind เลื่อน Iterator กลับไปยังรายการแรกใน Skiplist
func (it *Iterator[K, V]) Rewind() {
	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}
	it.current = it.sl.header.forward[0]
}

// Seek positions the iterator at the first element with a key greater than or equal to the given key.
// If no such element is found, the iterator becomes invalid.
// Seek เลื่อน Iterator ไปยังรายการที่มี key เท่ากับหรือมากกว่า key ที่กำหนด
// หากไม่พบ key ที่ตรงกัน จะเลื่อนไปที่รายการแรกที่มี key มากกว่า key ที่กำหนด
// หากไม่มีรายการใดๆ ที่ตรงตามเงื่อนไข จะทำให้ Iterator ไม่ Valid
func (it *Iterator[K, V]) Seek(key K) {
	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}

	current := it.sl.header
	// ค้นหาตำแหน่งที่จะเริ่ม
	for i := it.sl.level; i >= 0; i-- { // [Minor: This loop condition was missing in the original diff, adding it for completeness]
		for current.forward[i] != nil && it.sl.compare(current.forward[i].Key, key) < 0 {
			current = current.forward[i]
		}
	}
	// เลื่อนไปยังโหนดแรกที่มี key >= key ที่ต้องการ
	it.current = current.forward[0]
}
