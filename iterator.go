package skiplist

import "sync/atomic"

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
	reverse bool
	unsafe  bool // ถ้าเป็น true, จะไม่ทำการ lock/unlock (ใช้สำหรับ RangeWithIterator)
	// Optional inclusive end bound for iteration. If set, Next() stops before any key > end.
	end    K
	hasEnd bool
	// If non-zero, the iterator holds sl.mutex.RLock() and Close() must be called to release it.
	// Use an atomic uint32 to make Close() safe against concurrent Close() calls.
	lockHeld uint32
}

// IteratorOption configures an Iterator.
// IteratorOption คือฟังก์ชันสำหรับกำหนดค่าของ Iterator
type IteratorOption[K any, V any] func(*Iterator[K, V])

// withUnsafe creates an iterator that does not perform locking on its methods.
// This is intended for internal use cases like RangeWithIterator where the lock
// is already held.
func withUnsafe[K any, V any]() IteratorOption[K, V] {
	return func(it *Iterator[K, V]) {
		it.unsafe = true
	}
}

// WithEnd sets an inclusive upper bound for iteration. Iteration will include all elements.
// with key <= end and will stop when encountering the first element with key > end.
func WithEnd[K any, V any](end K) IteratorOption[K, V] {
	return func(it *Iterator[K, V]) {
		it.end = end
		it.hasEnd = true
	}
}

// WithReverse creates an iterator that iterates from the last element to the first.
// The standard `for it.Next() { ... }` loop will work in reverse.
func WithReverse[K any, V any]() IteratorOption[K, V] {
	return func(it *Iterator[K, V]) {
		it.reverse = true
	}
}

// NewIterator creates a new iterator. By default, it's positioned before the
// first element and iterates forwards. Options can be provided to change this behavior.
// A call to Next() is required to advance to the first (or last, if reversed) element.
// NewIterator สร้างและคืนค่า Iterator ใหม่ โดยปกติจะชี้ไปยังตำแหน่งก่อนรายการแรก
// และวนลูปไปข้างหน้า สามารถใช้ options เพื่อเปลี่ยนพฤติกรรมนี้ได้
// ต้องเรียก Next() เพื่อเลื่อนไปยังรายการแรก (หรือรายการสุดท้ายหากเป็นแบบย้อนกลับ)
func (sl *SkipList[K, V]) NewIterator(opts ...IteratorOption[K, V]) *Iterator[K, V] {
	it := &Iterator[K, V]{
		sl:      sl,
		current: sl.header, // Default start: before the first element
		unsafe:  false,
		reverse: false,
	}

	for _, opt := range opts {
		opt(it)
	}

	if it.reverse {
		// For reverse iteration, the "start" position is after the last element,
		// which we represent as a nil `current` pointer. The first call to Next()
		// will then move it to the actual last element.
		it.current = nil
	}

	return it
}

// Next moves the iterator to the next element and returns true if the move was successful.
// If the iterator was created with WithReverse, "next" means the previous element in the list.
// It returns false if there are no more elements.
// Next เลื่อน Iterator ไปยังรายการถัดไป และคืนค่า true หากสำเร็จ
// หาก Iterator ถูกสร้างด้วย WithReverse, "ถัดไป" จะหมายถึงรายการ "ก่อนหน้า"
// คืนค่า false หากไม่มีรายการเหลือแล้ว
func (it *Iterator[K, V]) Next() bool {
	if it.reverse {
		if !it.unsafe {
			it.sl.mutex.RLock()
			defer it.sl.mutex.RUnlock()
		}

		// If current is nil, it's the start of reverse iteration.
		// Position at the last element by calling the unlocked Last().
		if it.current == nil {
			if !it.lastInternal() {
				return false
			}
			// If an end bound is set, skip any trailing elements > end.
			if it.hasEnd {
				for {
					cur, _ := it.current.(*node[K, V])
					if cur == nil || cur == it.sl.header {
						it.current = nil
						return false
					}
					if it.sl.compare(cur.key, it.end) <= 0 {
						break
					}
					it.current = cur.backward
					if it.current == it.sl.header {
						it.current = nil
						return false
					}
				}
			}
			return true
		}

		// Otherwise, it's a standard "prev" move.
		currentNode, _ := it.current.(*node[K, V])
		// Already at the header (or somehow invalid), cannot move backward.
		if currentNode == it.sl.header {
			it.current = nil
			return false
		}

		it.current = currentNode.backward

		if it.current == it.sl.header {
			it.current = nil // Mark as exhausted
			return false
		}

		// If an end bound is set, skip backward over any nodes > end.
		if it.hasEnd {
			for {
				cur, _ := it.current.(*node[K, V])
				if cur == nil || cur == it.sl.header {
					it.current = nil
					return false
				}
				if it.sl.compare(cur.key, it.end) <= 0 {
					break
				}
				it.current = cur.backward
				if it.current == it.sl.header {
					it.current = nil
					return false
				}
			}
		}
		return true
	}

	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}

	// Safely get the concrete node pointer from the interface.
	// This correctly handles both a nil interface and an interface containing a nil pointer.
	currentNode, ok := it.current.(*node[K, V])
	if !ok || currentNode == nil {
		it.current = nil // Ensure iterator is marked as exhausted.
		return false
	}

	nextNode := currentNode.forward[0]
	if nextNode == nil {
		it.current = nil // Mark as exhausted by setting to a true nil interface.
		return false
	}
	// If an end bound is set, stop before visiting any element > end.
	if it.hasEnd {
		if it.sl.compare(nextNode.key, it.end) > 0 {
			it.current = nil
			return false
		}
	}
	it.current = nextNode
	return true
}

// Key returns the key of the element at the current iterator position.
// It should only be called after a call to Next() has returned true.
// Key คืนค่า key ของรายการปัจจุบันที่ Iterator ชี้อยู่
// ควรเรียกใช้หลังจากที่ Next(), First(), Last(), หรือ Seek() ที่สำเร็จ คืนค่า true เท่านั้น
// การเรียกใช้บน iterator ที่ไม่ถูกต้อง (เช่น สิ้นสุดไปแล้ว) จะทำให้เกิด panic
func (it *Iterator[K, V]) Key() K {
	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}
	// Check for invalid iterator state. The iterator is invalid if it's at the header
	// (before the first element) or exhausted (current is nil).
	if it.current == nil || it.current == it.sl.header {
		panic("skiplist: Key() called on exhausted or invalid iterator")
	}
	return it.current.Key()
}

// Value returns the value of the element at the current iterator position.
// It should only be called after a call to Next() has returned true.
// Value คืนค่า value ของรายการปัจจุบันที่ Iterator ชี้อยู่
// ควรเรียกใช้หลังจากที่ Next(), First(), Last(), หรือ Seek() ที่สำเร็จ คืนค่า true เท่านั้น
// การเรียกใช้บน iterator ที่ไม่ถูกต้อง (เช่น สิ้นสุดไปแล้ว) จะทำให้เกิด panic
func (it *Iterator[K, V]) Value() V {
	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}
	if it.current == nil || it.current == it.sl.header {
		panic("skiplist: Value() called on exhausted or invalid iterator")
	}
	return it.current.Value()
}

// Reset moves the iterator back to its initial state, before the first element.
// A subsequent call to Next() is required to advance to the first element.
// This method respects the iterator's direction (normal or reverse).
// Reset เลื่อน Iterator กลับไปยังสถานะเริ่มต้น (ก่อนรายการแรก)
// โดยจะเคารพทิศทางของ iterator (ปกติหรือย้อนกลับ)
// ต้องเรียก Next() อีกครั้งเพื่อเลื่อนไปยังรายการแรก
func (it *Iterator[K, V]) Reset() {
	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}
	if it.reverse {
		// The initial state for a reverse iterator is after the last element (nil).
		it.current = nil
	} else {
		it.current = it.sl.header
	}
}

// Prev moves the iterator to the previous element and returns true if the move was successful.
// If the iterator was created with WithReverse, "previous" means the next element in the list.
// It returns false if there are no more elements in that direction.
// Prev เลื่อน Iterator ไปยังรายการก่อนหน้า และคืนค่า true หากสำเร็จ
// หาก Iterator ถูกสร้างด้วย WithReverse, "ก่อนหน้า" จะหมายถึงรายการ "ถัดไป"
// คืนค่า false หากไม่มีรายการเหลือแล้วในทิศทางนั้น
func (it *Iterator[K, V]) Prev() bool {
	if it.reverse {
		// This is a forward move, which is the logic of the original Next().
		if !it.unsafe {
			it.sl.mutex.RLock()
			defer it.sl.mutex.RUnlock()
		}

		currentNode, ok := it.current.(*node[K, V])
		if !ok || currentNode == nil {
			it.current = nil
			return false
		}

		nextNode := currentNode.forward[0]
		if nextNode == nil {
			it.current = nil
			return false
		}
		it.current = nextNode
		return true
	}

	// Standard backward move.
	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}

	currentNode, _ := it.current.(*node[K, V])
	if currentNode == nil || currentNode == it.sl.header {
		it.current = nil
		return false
	}

	it.current = currentNode.backward

	if it.current == it.sl.header {
		it.current = nil
		return false
	}
	return true
}

// First moves the iterator to the first element in the skiplist.
// This is the element with the smallest key, regardless of the iterator's direction.
// It returns true if a first element exists, otherwise it returns false.
// After a call to First(), Key() and Value() will return the data of the first element.
// First เลื่อน Iterator ไปยังรายการแรกใน Skiplist
// (รายการที่มี key น้อยที่สุด) โดยไม่สนใจทิศทางของ iterator
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

// lastInternal is an unlocked version of Last() for internal use.
func (it *Iterator[K, V]) lastInternal() bool {
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

// Last moves the iterator to the last element in the skiplist.
// This is the element with the largest key, regardless of the iterator's direction.
// It returns true if a last element exists, otherwise it returns false.
// After a call to Last(), Key() and Value() will return the data of the last element.
// Last เลื่อน Iterator ไปยังรายการสุดท้ายใน Skiplist
// (รายการที่มี key มากที่สุด) โดยไม่สนใจทิศทางของ iterator
// คืนค่า true หากมีรายการสุดท้ายอยู่, มิฉะนั้นคืนค่า false
// หลังจากเรียก Last(), Key() และ Value() จะคืนค่าของรายการสุดท้าย
func (it *Iterator[K, V]) Last() bool {
	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}
	return it.lastInternal()
}

// Close releases any lock held by the iterator. It is a no-op for iterators
// that do not hold the read lock. Call Close() when you are finished iterating
// if the iterator was obtained from `SkipList.RangeIterator(start,end)` which holds
// the read lock for the iterator's lifetime.
func (it *Iterator[K, V]) Close() {
	// Atomically clear the flag; only the goroutine that observes a 1
	// needs to call RUnlock(). This prevents double-unlock when Close()
	// is called concurrently from multiple goroutines.
	if atomic.SwapUint32(&it.lockHeld, 0) == 1 {
		it.sl.mutex.RUnlock()
	}
}

// SeekToFirst positions the iterator just before the first element.
// For a forward iterator, this is before the smallest key.
// For a reverse iterator, this is before the largest key.
// A subsequent call to Next() will advance the iterator to the first element.
// It returns true if the list is not empty, indicating that a subsequent
// call to Next() will succeed.
// SeekToFirst เลื่อน Iterator ไปยังตำแหน่งก่อนหน้าของรายการแรกในลำดับการวนลูป
// สำหรับ iterator ไปข้างหน้า: คือก่อน key ที่น้อยที่สุด
// สำหรับ iterator ย้อนกลับ: คือก่อน key ที่มากที่สุด
// คืนค่า true หาก list ไม่ว่างเปล่า เพื่อบ่งชี้ว่าการเรียก Next() ครั้งถัดไปจะสำเร็จ
func (it *Iterator[K, V]) SeekToFirst() bool {
	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}
	if it.reverse {
		// For reverse iteration, the "start" position is after the last element,
		// which we represent as a nil `current` pointer.
		it.current = nil
	} else {
		// For forward iteration, the start position is the header node.
		it.current = it.sl.header
	}
	// Return true if a first element exists to be iterated over.
	return it.sl.length > 0
}

// SeekToLast positions the iterator just before the last element in the skiplist (the one with the largest key).
// This behavior is consistent regardless of the iterator's direction (normal or reverse).
// A subsequent call to Next() will advance the iterator to the last element.
// It returns true if the list is not empty.
//
// SeekToLast เลื่อน Iterator ไปยังตำแหน่งก่อนหน้าของรายการสุดท้ายใน Skiplist (รายการที่มี key สูงสุด)
// พฤติกรรมนี้จะเหมือนกันเสมอ ไม่ว่า iterator จะเป็นแบบปกติหรือแบบย้อนกลับ (reverse)
// การเรียก Next() หลังจากนี้จะเลื่อนไปยังรายการสุดท้าย
func (it *Iterator[K, V]) SeekToLast() bool {
	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}

	// Find the last node, same logic as Last()
	lastNode := it.sl.header
	for i := it.sl.level; i >= 0; i-- {
		for lastNode.forward[i] != nil {
			lastNode = lastNode.forward[i]
		}
	}

	// If the list is empty, lastNode is the header. Position before the start.
	if lastNode == it.sl.header {
		it.current = it.sl.header
		return false
	}

	// The node before the last node is its backward pointer.
	// This will correctly be the header if there is only one element.
	it.current = lastNode.backward
	return true
}

// Seek moves the iterator to the first element with a key greater than or equal to the given key.
// This behavior is consistent regardless of the iterator's direction (normal or reverse).
// It returns true if such an element is found, otherwise it returns false and the iterator is positioned at the end.
// After a successful seek, Key() and Value() will return the data of the found element.
// Seek เลื่อน Iterator ไปยังรายการแรกที่มี key เท่ากับหรือมากกว่า key ที่กำหนด
// พฤติกรรมนี้จะเหมือนกันเสมอ ไม่ว่า iterator จะเป็นแบบปกติหรือแบบย้อนกลับ (reverse)
// คืนค่า true หากพบรายการดังกล่าว, มิฉะนั้นคืนค่า false และ Iterator จะชี้ไปที่ท้ายสุด
// หลังจาก seek สำเร็จ, Key() และ Value() จะคืนค่าของรายการที่พบ
func (it *Iterator[K, V]) Seek(key K) bool {
	if !it.unsafe {
		it.sl.mutex.RLock()
		defer it.sl.mutex.RUnlock()
	}

	// Reuse SkipList's findGreaterOrEqual for the correct ceiling node logic.
	found := it.findGreaterOrEqual(key)

	// If an end bound is set and the found node is beyond the end, treat as not found.
	if found != nil && it.hasEnd {
		if it.sl.compare(found.key, it.end) > 0 {
			it.current = nil
			return false
		}
	}

	// Position the iterator at the found node (so Key()/Value() return it).
	it.current = found
	return found != nil
}

func (it *Iterator[K, V]) findGreaterOrEqual(key K) *node[K, V] {
	current := it.sl.header
	for i := it.sl.level; i >= 0; i-- {
		for current.forward[i] != nil && it.sl.compare(current.forward[i].key, key) < 0 {
			current = current.forward[i]
		}
	}
	// The next node is the first one with a key >= key.
	return current.forward[0]
}

// Clone creates an independent copy of the iterator at its current position.
// The new iterator can be moved independently of the original.
// Clone สร้างสำเนาของ Iterator ณ ตำแหน่งปัจจุบัน
// Iterator ที่สร้างขึ้นใหม่จะทำงานเป็นอิสระจากตัวต้นฉบับ
func (it *Iterator[K, V]) Clone() *Iterator[K, V] {
	// A shallow copy is sufficient as the underlying skiplist is shared,
	// and the iterator's state is just a pointer and flags.
	return &Iterator[K, V]{
		sl:      it.sl,
		current: it.current,
		unsafe:  it.unsafe,
		reverse: it.reverse,
	}
}
