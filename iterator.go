package skiplist

// Iterator provides a way to iterate over the elements of a SkipList.
// The typical use is:
//
//	it := sl.NewIterator()
//	for it.Next() {
//		key := it.Key()
//		value := it.Value()
//		// ...
//	}
//
// Iterator คือโครงสร้างที่ใช้สำหรับวนลูปผ่านรายการใน Skiplist
// รูปแบบการใช้งานทั่วไป:
//
//	it := sl.NewIterator()
//	for it.Next() {
//		key := it.Key()
//		value := it.Value()
//		// ...
//	}
type Iterator[K any, V any] struct {
	sl      *SkipList[K, V] // อ้างอิงถึง Skiplist ที่กำลังวนลูป
	current INode[K, V]     // โหนดปัจจุบันที่ Iterator ชี้อยู่
	unsafe  bool            // ถ้าเป็น true, จะไม่ทำการ lock/unlock (ใช้สำหรับ RangeWithIterator)
}

// NewIterator creates a new iterator positioned at the first element of the skiplist.
// A call to Next() is required to advance to the first element.
// NewIterator สร้างและคืนค่า Iterator ใหม่ที่ชี้ไปยังตำแหน่งก่อนรายการแรกใน Skiplist
// ต้องเรียก Next() เพื่อเลื่อนไปยังรายการแรก
func (sl *SkipList[K, V]) NewIterator() *Iterator[K, V] {
	// สร้าง iterator แบบ thread-safe ปกติ
	// เริ่มต้นที่ header node เพื่อให้การเรียก Next() ครั้งแรกเลื่อนไปยังโหนดแรกจริงๆ
	return &Iterator[K, V]{
		sl:      sl,
		current: sl.header, // เริ่มต้นที่โหนด header
		unsafe:  false,
	}
}

// Next moves the iterator to the next element and returns true if the move was successful.
// It returns false if there are no more elements.
// Next เลื่อน Iterator ไปยังรายการถัดไป และคืนค่า true หากสำเร็จ
// คืนค่า false หากไม่มีรายการเหลือแล้ว
func (it *Iterator[K, V]) Next() bool {
	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}
	if it.current == nil {
		return false
	}
	// We must cast to the concrete type to access the forward pointer.
	// Then, we check the concrete pointer for nilness, not the interface.
	// This avoids the `(type, nil)` interface problem where `it.current != nil` could be true
	// even if the underlying pointer is nil, causing a panic on the next Key/Value call.
	nextNode := it.current.(*node[K, V]).forward[0]
	if nextNode == nil {
		it.current = nil // Mark as exhausted by setting to a true nil interface.
		return false
	}
	it.current = nextNode
	return true
}

// Key returns the key of the element at the current iterator position.
// It should only be called after a call to Next() has returned true.
// Key คืนค่า key ของรายการปัจจุบันที่ Iterator ชี้อยู่
// ควรเรียกใช้หลังจากที่ Next() คืนค่า true เท่านั้น
func (it *Iterator[K, V]) Key() K {
	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}
	// No nil check needed here because it's a contract violation to call Key()
	// if Next() hasn't returned true. The current node is guaranteed to be non-nil.
	return it.current.Key()
}

// Value returns the value of the element at the current iterator position.
// It should only be called after a call to Next() has returned true.
// Value คืนค่า value ของรายการปัจจุบันที่ Iterator ชี้อยู่
// ควรเรียกใช้หลังจากที่ Next() คืนค่า true เท่านั้น
func (it *Iterator[K, V]) Value() V {
	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}
	// No nil check needed here.
	return it.current.Value()
}

// Reset moves the iterator back to its initial state, before the first element.
// A subsequent call to Next() is required to advance to the first element.
// Reset เลื่อน Iterator กลับไปยังสถานะเริ่มต้น (ก่อนรายการแรก)
// ต้องเรียก Next() อีกครั้งเพื่อเลื่อนไปยังรายการแรก
func (it *Iterator[K, V]) Reset() {
	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}
	it.current = it.sl.header
}

// Prev moves the iterator to the previous element and returns true if the move was successful.
// It returns false if there are no more elements in that direction.
// To begin reverse iteration, first position the iterator at the end using Last().
//
// Prev เลื่อน Iterator ไปยังรายการก่อนหน้า และคืนค่า true หากสำเร็จ
// คืนค่า false หากไม่มีรายการเหลือแล้วในทิศทางนั้น (เช่น อยู่ที่รายการแรกแล้ว)
// หากต้องการเริ่มวนลูปย้อนกลับ, ให้ใช้ Last() เพื่อไปยังท้ายสุดก่อน
func (it *Iterator[K, V]) Prev() bool {
	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}

	currentNode, _ := it.current.(*node[K, V])
	// If the iterator is exhausted (nil) or at the header (which means it's positioned
	// before the first element), we cannot move backward.
	if currentNode == nil || currentNode == it.sl.header {
		return false
	}

	// Move to the previous node. This could be the header node.
	it.current = currentNode.backward

	// The move is successful only if the new position is a valid data node.
	// If we've moved to the header, it means we've gone past the beginning of the list.
	// We set current to nil to signal exhaustion, consistent with Next().
	if it.current == it.sl.header {
		it.current = nil
		return false
	}
	return true
}

// First moves the iterator to the first element in the skiplist.
// It returns true if a first element exists, otherwise it returns false.
// After a call to First(), Key() and Value() will return the data of the first element.
// First เลื่อน Iterator ไปยังรายการแรกใน Skiplist
// คืนค่า true หากมีรายการแรกอยู่, มิฉะนั้นคืนค่า false
// หลังจากเรียก First(), Key() และ Value() จะคืนค่าของรายการแรก
func (it *Iterator[K, V]) First() bool {
	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}
	firstNode := it.sl.header.forward[0]
	it.current = firstNode
	return firstNode != nil
}

// Last moves the iterator to the last element in the skiplist.
// It returns true if a last element exists, otherwise it returns false.
// After a call to Last(), Key() and Value() will return the data of the last element.
// Last เลื่อน Iterator ไปยังรายการสุดท้ายใน Skiplist
// คืนค่า true หากมีรายการสุดท้ายอยู่, มิฉะนั้นคืนค่า false
// หลังจากเรียก Last(), Key() และ Value() จะคืนค่าของรายการสุดท้าย
func (it *Iterator[K, V]) Last() bool {
	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}

	// The logic here is identical to SkipList.Max()
	current := it.sl.header
	for i := it.sl.level; i >= 0; i-- {
		for current.forward[i] != nil {
			current = current.forward[i]
		}
	}

	if current == it.sl.header {
		// List is empty
		it.current = nil
		return false
	}

	it.current = current
	return true
}

// Seek positions the iterator just before the first element with a key greater than or equal to the given key.
// A subsequent call to Next() will advance the iterator to that element.
// Seek เลื่อน Iterator ไปยังตำแหน่งก่อนหน้าของรายการที่มี key เท่ากับหรือมากกว่า key ที่กำหนด
// การเรียก Next() หลังจากนี้จะเลื่อนไปยังรายการนั้น
func (it *Iterator[K, V]) Seek(key K) {
	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}

	current := it.sl.header
	// ค้นหาตำแหน่งที่จะเริ่ม
	for i := it.sl.level; i >= 0; i-- {
		for current.forward[i] != nil && it.sl.compare(current.forward[i].key, key) < 0 {
			current = current.forward[i]
		}
	}
	// ตั้งค่า current ให้เป็นโหนดก่อนหน้าโหนดเป้าหมาย
	// เพื่อให้การเรียก Next() ครั้งถัดไปเลื่อนไปยังโหนดเป้าหมายพอดี
	it.current = current
}

// Clone creates an independent copy of the iterator at its current position.
// The new iterator can be moved independently of the original.
// Clone สร้างสำเนาของ Iterator ณ ตำแหน่งปัจจุบัน
// Iterator ที่สร้างขึ้นใหม่จะทำงานเป็นอิสระจากตัวต้นฉบับ
func (it *Iterator[K, V]) Clone() *Iterator[K, V] {
	// A shallow copy is sufficient as the underlying skiplist is shared
	// and the iterator's state is just a pointer.
	return &Iterator[K, V]{
		sl:      it.sl,
		current: it.current,
		unsafe:  it.unsafe,
	}
}
