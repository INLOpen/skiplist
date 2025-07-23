// package skiplist implements a thread-safe, generic skiplist.
// A skiplist is a probabilistic data structure that allows for fast search,
// insertion, and deletion of elements in a sorted sequence.
// It provides O(log n) performance on average for these operations.
package skiplist

import (
	"cmp" // Re-add cmp for default comparator
	"math/rand/v2"
	"sync"
)

const (
	// MaxLevel is the maximum number of levels a skiplist can have.
	// A value of 32 is sufficient for approximately 2^32 (4 billion) items.
	// MaxLevel คือจำนวนชั้นสูงสุดที่ skiplist สามารถมีได้
	// ค่า 32 เพียงพอสำหรับข้อมูลประมาณ 2^32 (4 พันล้าน) รายการ
	MaxLevel = 32
	// P is the probability used to determine the level of a new node.
	// A value of 0.25 is commonly used and provides good performance.
	// P คือค่าความน่าจะเป็นในการเพิ่มชั้นของโหนดใหม่
	// ค่า 0.25 เป็นค่าที่นิยมใช้และให้ประสิทธิภาพที่ดี
	P = 0.25
)

// Comparator is a function that compares two keys.
// It should return:
//   - a negative value if a < b
//   - zero if a == b
//   - a positive value if a > b
//
// Comparator คือฟังก์ชันสำหรับเปรียบเทียบ key สองตัว
// ควรคืนค่า:
//
//	< 0 ถ้า a น้อยกว่า b
//	0 ถ้า a เท่ากับ b
//	> 0 ถ้า a มากกว่า b
type Comparator[K any] func(a, b K) int

// SkipList represents a thread-safe skiplist data structure.
// The zero value for a SkipList is not ready to use; one of the New functions must be called.
// SkipList คือโครงสร้างหลักของ skiplist
// ค่า zero value ของ SkipList จะยังไม่พร้อมใช้งาน, ต้องสร้างผ่านฟังก์ชัน New... เท่านั้น
type SkipList[K any, V any] struct {
	header      *node[K, V]         // โหนดเริ่มต้น (sentinel node)
	level       int                 // ชั้นสูงสุดที่มีอยู่ในปัจจุบัน
	length      int                 // จำนวนรายการทั้งหมดใน skiplist
	rand        *rand.Rand          // ตัวสร้างเลขสุ่มสำหรับกำหนดชั้น
	mutex       sync.RWMutex        // Mutex สำหรับการทำงานแบบ concurrent-safe
	updateCache []INode[K, V]       // แคชสำหรับ update path
	allocator   nodeAllocator[K, V] // Abstraction สำหรับการจัดสรรหน่วยความจำ
	compare     Comparator[K]       // ฟังก์ชันสำหรับเปรียบเทียบ key
}

// Option is a function that configures a SkipList.
// Option คือฟังก์ชันสำหรับกำหนดค่าของ SkipList
type Option[K any, V any] func(*SkipList[K, V])

// WithArena configures the SkipList to use a memory arena of a given size in bytes.
// This is a high-performance option that reduces GC pressure.
// WithArena กำหนดให้ SkipList ใช้ memory arena ตามขนาดที่ระบุ (เป็น byte)
// เป็นตัวเลือกประสิทธิภาพสูงที่ช่วยลดภาระของ GC
func WithArena[K any, V any](sizeInBytes int) Option[K, V] {
	return func(sl *SkipList[K, V]) {
		sl.allocator = newArenaAllocator[K, V](sizeInBytes)
	}
}

// New creates a new skiplist for key types that implement cmp.Ordered (e.g., int, string).
// It uses cmp.Compare as the default comparator.
// New สร้าง skiplist ใหม่สำหรับ key type ที่รองรับ `cmp.Ordered` (เช่น int, string)
// โดยจะใช้ `cmp.Compare` เป็นฟังก์ชันเปรียบเทียบโดยอัตโนมัติ
func New[K cmp.Ordered, V any](opts ...Option[K, V]) *SkipList[K, V] {
	return NewWithComparator(cmp.Compare[K], opts...)
}

// NewWithComparator creates a new skiplist with a custom comparator function.
// This is suitable for key types that do not implement cmp.Ordered or require special sorting logic.
// The comparator function must not be nil.
// NewWithComparator สร้าง skiplist ใหม่พร้อมกับฟังก์ชันเปรียบเทียบที่กำหนดเอง
// เหมาะสำหรับ key type ที่ไม่รองรับ `cmp.Ordered` หรือต้องการการเรียงลำดับแบบพิเศษ
func NewWithComparator[K any, V any](compare Comparator[K], opts ...Option[K, V]) *SkipList[K, V] {
	if compare == nil {
		panic("skiplist: comparator cannot be nil")
	}

	// สร้าง header node ซึ่งเป็นโหนดเริ่มต้นที่ไม่เก็บข้อมูลจริง
	// แต่มีตัวชี้ครบทุกชั้น
	header := &node[K, V]{
		forward: make([]*node[K, V], MaxLevel),
	}

	// ใช้ PCG (Permuted Congruential Generator) ซึ่งเป็น default ใน Go 1.22+
	// เพื่อประสิทธิภาพที่ดีกว่า
	source := rand.NewPCG(rand.Uint64(), rand.Uint64())

	sl := &SkipList[K, V]{
		header:      header,
		level:       0, // เริ่มต้นที่ชั้น 0
		length:      0,
		rand:        rand.New(source),
		updateCache: make([]INode[K, V], MaxLevel),
		allocator:   newPoolAllocator[K, V](), // Default to sync.Pool
		compare:     compare,
	}

	// Apply any custom options provided by the user
	for _, opt := range opts {
		opt(sl)
	}
	return sl
}

// randomLevel สุ่มความสูง (จำนวนชั้น) ของโหนดใหม่
// โดยใช้วิธี bit-manipulation เพื่อประสิทธิภาพที่สูงขึ้น
func (sl *SkipList[K, V]) randomLevel() int {
	// เราใช้ประโยชน์จากข้อเท็จจริงที่ว่า P = 0.25 (1/4)
	// โดยการตรวจสอบ 2 บิต จากเลขสุ่ม 64-bit ในแต่ละครั้ง
	// การทำเช่นนี้จะเร็วกว่าการเรียก sl.rand.Float64() ในลูป
	// เพราะหลีกเลี่ยงการคำนวณเลขทศนิยมและลดการเรียกฟังก์ชัน
	//
	// x&3 == 0 จะให้โอกาส 1 ใน 4 (สำหรับ 00, 01, 10, 11)
	// ซึ่งตรงกับค่า P ของเรา
	x := sl.rand.Uint64()
	level := 1
	for (x&3) == 0 && level < MaxLevel {
		level++
		x >>= 2 // เลื่อนไปตรวจสอบ 2 บิตถัดไป
	}
	return level
}

// Search ค้นหา value จาก key ที่กำหนด
// Search searches for a value by its key.
// It returns the node and true if the key is found, otherwise it returns nil and false.
// คืนค่าโหนดและ true หากพบ, มิฉะนั้นคืนค่า nil และ false
func (sl *SkipList[K, V]) Search(key K) (INode[K, V], bool) {
	sl.mutex.RLock()
	defer sl.mutex.RUnlock()

	current := sl.header

	// เริ่มค้นหาจากชั้นบนสุดลงมา
	for i := sl.level; i >= 0; i-- {
		// วิ่งไปข้างหน้าในชั้นปัจจุบันจนกว่าโหนดถัดไปจะมี key มากกว่าหรือเท่ากับ key ที่ค้นหา
		for current.forward[i] != nil && sl.compare(current.forward[i].key, key) < 0 {
			current = current.forward[i]
		}
	}

	// หลังจากลูปสิ้นสุด, current จะอยู่ที่โหนดก่อนหน้าโหนดที่อาจจะตรงกับ key
	// เราจึงต้องเลื่อนไปข้างหน้าอีกหนึ่งตำแหน่งที่ชั้นล่างสุด (level 0)
	current = current.forward[0]

	// ตรวจสอบว่าโหนดปัจจุบันคือโหนดที่ต้องการหรือไม่
	if current != nil && sl.compare(current.key, key) == 0 {
		return current, true
	}

	return nil, false
}

// Insert เพิ่ม key-value คู่ใหม่เข้าไปใน skiplist
// Insert adds a new key-value pair to the skiplist.
// If the key already exists, its value is updated, and the old node is returned.
// If the key is new, a new node is inserted and nil is returned.
// หาก key มีอยู่แล้ว จะทำการอัปเดต value และคืนค่าโหนดเก่า
// หากเป็น key ใหม่ จะเพิ่มโหนดใหม่และคืนค่า nil
func (sl *SkipList[K, V]) Insert(key K, value V) INode[K, V] {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	// update เป็น slice ที่เก็บโหนดที่จะต้องอัปเดตตัวชี้ forward
	// ในแต่ละชั้นเมื่อมีการเพิ่มโหนดใหม่
	update := sl.updateCache
	current := sl.header

	// ค้นหาตำแหน่งที่จะเพิ่มโหนดใหม่ พร้อมทั้งบันทึกโหนดที่จะต้องอัปเดต
	for i := sl.level; i >= 0; i-- {
		for current.forward[i] != nil && sl.compare(current.forward[i].key, key) < 0 {
			current = current.forward[i]
		}
		update[i] = current
	}

	current = current.forward[0]

	// ถ้า key มีอยู่แล้ว ให้อัปเดต value แล้วจบการทำงาน
	if current != nil && sl.compare(current.key, key) == 0 {
		old := current
		current.value = value
		return old
	}

	// ถ้า key ยังไม่มีอยู่ ให้สร้างโหนดใหม่
	newLevel := sl.randomLevel()

	// หากชั้นที่สุ่มได้สูงกว่าชั้นสูงสุดปัจจุบันของ skiplist
	// ให้อัปเดตชั้นสูงสุดและตั้งค่า update สำหรับชั้นใหม่ๆ ให้ชี้มาจาก header
	// newLevel is 1-based, sl.level is 0-based
	if newLevel-1 > sl.level {
		for i := sl.level + 1; i < newLevel; i++ {
			update[i] = sl.header
		}
		sl.level = newLevel - 1
	}

	// --- จัดสรรโหนดโดยใช้ Allocator ที่กำหนดไว้ ---
	newNode := sl.allocator.Get()

	// ตรวจสอบและปรับขนาดของ forward slice ให้เหมาะสมกับ level ใหม่
	// สำหรับ Arena, `Get` จะคืนโหนดที่ `forward` เป็น nil และต้องสร้างใหม่เสมอ
	// สำหรับ Pool, `Get` จะคืนโหนดที่อาจมี slice เก่ามาด้วย ซึ่งเราสามารถใช้ซ้ำได้
	if cap(newNode.forward) < newLevel {
		newNode.forward = make([]*node[K, V], newLevel)
	} else {
		newNode.forward = newNode.forward[:newLevel]
	}

	newNode.key = key
	newNode.value = value

	// เชื่อมโหนดใหม่เข้ากับ skiplist ในแต่ละชั้น
	for i := 0; i < newLevel; i++ {
		cupdate, ok := update[i].(*node[K, V])
		if !ok {
			continue
		}
		newNode.forward[i] = cupdate.forward[i]
		cupdate.forward[i] = newNode
	}

	// ตั้งค่า backward pointer สำหรับ doubly-linked list ที่ชั้น 0
	// Set up backward pointer for the doubly-linked list at level 0
	newNode.backward = update[0].(*node[K, V])
	if newNode.forward[0] != nil {
		newNode.forward[0].backward = newNode
	}

	sl.length++
	return nil
}

// deleteNode เป็น helper ภายในที่จัดการตรรกะการลบโหนด
// โดยจะถูกเรียกจาก Delete, PopMin, และ PopMax
// **หมายเหตุ**: ผู้เรียกต้องถือ write lock (sl.mutex.Lock()) อยู่แล้ว
func (sl *SkipList[K, V]) deleteNode(nodeToRemove INode[K, V], update []INode[K, V]) {
	// อัปเดตตัวชี้ forward ในแต่ละชั้นเพื่อ "ข้าม" โหนดที่ถูกลบไป
	cnodeRemove, ok := nodeToRemove.(*node[K, V])
	if !ok {
		return
	}

	for i := 0; i <= sl.level; i++ {
		cupdate, ok1 := update[i].(*node[K, V])
		if !ok1 {
			continue
		}

		if cupdate.forward[i] != cnodeRemove {
			break
		}
		cupdate.forward[i] = cnodeRemove.forward[i]
	}

	// ลดระดับของ skiplist หากชั้นบนสุดว่างลง
	for sl.level > 0 && sl.header.forward[sl.level] == nil {
		sl.level--
	}

	// อัปเดต backward pointer ของโหนดถัดไป (ถ้ามี)
	// Update the backward pointer of the next node, if it exists.
	if cnodeRemove.forward[0] != nil {
		cnodeRemove.forward[0].backward = cnodeRemove.backward
	}

	// คืนโหนดกลับเข้า Allocator
	// สำหรับ Arena, Put() อาจจะไม่ทำอะไรเลย เพราะหน่วยความจำจะถูกเคลียร์ทีเดียวตอน Reset()
	// สำหรับ Pool, Put() จะทำการเคลียร์ค่าและคืนโหนดกลับเข้า Pool
	sl.allocator.Put(cnodeRemove)

	sl.length--
}

// Delete ลบ key-value ออกจาก skiplist
// Delete removes a key-value pair from the skiplist.
// It returns true if the key was found and removed, otherwise false.
// คืนค่า true หากลบสำเร็จ, false หากไม่พบ key
func (sl *SkipList[K, V]) Delete(key K) bool {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	update := sl.updateCache
	current := sl.header

	// ค้นหาโหนดที่จะลบ พร้อมทั้งบันทึกโหนดที่จะต้องอัปเดต
	for i := sl.level; i >= 0; i-- {
		for current.forward[i] != nil && sl.compare(current.forward[i].key, key) < 0 {
			current = current.forward[i]
		}
		update[i] = current
	}

	current = current.forward[0]

	// ถ้าพบโหนดที่ต้องการลบ
	if current != nil && sl.compare(current.key, key) == 0 {
		sl.deleteNode(current, update)
		return true
	}

	// ไม่พบ key ที่ต้องการลบ
	return false
}

// Clear removes all items from the skiplist, resetting it to an empty state.
// It also replaces the internal node pool, allowing the garbage collector to reclaim
// memory from the old nodes. This is useful to free up memory after the skiplist
// is no longer needed, or before reusing it for a different dataset.
//
// Clear ลบรายการทั้งหมดออกจาก skiplist และรีเซ็ตให้อยู่ในสถานะว่างเปล่า
// และยังทำการแทนที่ node pool ภายใน, ทำให้ garbage collector สามารถคืนหน่วยความจำ
// จากโหนดเก่าได้. มีประโยชน์ในการคืนหน่วยความจำหลังจากที่ skiplist ไม่ได้ใช้งานแล้ว
// หรือก่อนที่จะนำไปใช้กับข้อมูลชุดใหม่
func (sl *SkipList[K, V]) Clear() {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	// Reset the skiplist's structural properties
	sl.level = 0
	sl.length = 0
	for i := range sl.header.forward {
		sl.header.forward[i] = nil
	}

	// Reset the allocator.
	// For Arena, this reclaims all memory.
	// For Pool, we replace it to allow the old one to be GC'd.
	if _, ok := sl.allocator.(*arenaAllocator[K, V]); ok {
		sl.allocator.Reset()
	} else {
		sl.allocator = newPoolAllocator[K, V]()
	}
}

// Len คืนค่าจำนวนรายการทั้งหมดใน skiplist
// Len returns the total number of items in the skiplist.
func (sl *SkipList[K, V]) Len() int {
	sl.mutex.RLock()
	defer sl.mutex.RUnlock()
	return sl.length
}

// Range วนลูปไปตามรายการทั้งหมดใน skiplist ตามลำดับ key
// Range iterates over all items in the skiplist in ascending key order.
// The iteration stops if the provided function f returns false.
// และเรียกใช้ฟังก์ชัน f สำหรับแต่ละคู่ key-value
// การวนลูปจะหยุดลงหากฟังก์ชัน f คืนค่า false
func (sl *SkipList[K, V]) Range(f func(key K, value V) bool) {
	sl.mutex.RLock()
	defer sl.mutex.RUnlock()

	current := sl.header.forward[0]
	for current != nil {
		if !f(current.key, current.value) {
			break
		}
		current = current.forward[0]
	}
}

// RangeWithIterator provides a locked iterator to a callback function.
// This is more efficient than creating a new iterator and manually locking,
// as it acquires a single read lock for the entire duration of the callback's execution.
// The iterator provided to the callback is only valid within the scope of that callback.
//
// RangeWithIterator ให้ Iterator ที่ถูก lock ไปยัง callback function
// ซึ่งมีประสิทธิภาพสูงกว่าการสร้าง Iterator แล้ว lock ด้วยตนเอง
// เพราะจะทำการ RLock เพียงครั้งเดียวตลอดการทำงานของ callback
// Iterator ที่ได้มาจะสามารถใช้งานได้ภายใน callback เท่านั้น
func (sl *SkipList[K, V]) RangeWithIterator(f func(it *Iterator[K, V])) {
	sl.mutex.RLock()
	defer sl.mutex.RUnlock()

	// สร้าง iterator แบบ "unsafe" ที่ไม่ทำการ lock ภายในตัวเอง
	// เพราะเรา lock จากภายนอกแล้ว
	it := &Iterator[K, V]{
		sl:      sl,
		current: sl.header, // เริ่มต้นที่ header เพื่อให้ it.Next() ทำงานถูกต้อง
		unsafe:  true,
	}
	f(it)
}

// Min คืนค่า key-value คู่แรก (น้อยที่สุด) ใน skiplist
// Min returns the first (smallest) key-value pair in the skiplist.
// It returns the node and true if the list is not empty, otherwise it returns nil and false.
// จะคืนค่าโหนดและ true หาก list ไม่ว่าง, มิฉะนั้นคืนค่า nil และ false
func (sl *SkipList[K, V]) Min() (INode[K, V], bool) {
	sl.mutex.RLock()
	defer sl.mutex.RUnlock()

	if sl.length == 0 {
		return nil, false
	}

	firstNode := sl.header.forward[0]
	return firstNode, true
}

// Max คืนค่า key-value คู่สุดท้าย (มากที่สุด) ใน skiplist
// Max returns the last (largest) key-value pair in the skiplist.
// It returns the nil for Node and false if the skiplist is empty.
// จะคืนค่า nil และ false หาก skiplist ว่างเปล่า
func (sl *SkipList[K, V]) Max() (INode[K, V], bool) {
	sl.mutex.RLock()
	defer sl.mutex.RUnlock()

	if sl.length == 0 {
		return nil, false
	}

	current := sl.header
	// วิ่งไปทางขวาสุดในทุกชั้นจากบนลงล่าง
	for i := sl.level; i >= 0; i-- {
		for current.forward[i] != nil {
			current = current.forward[i]
		}
	}

	return current, true
}

// RangeQuery วนลูปไปตามรายการที่ key อยู่ระหว่าง start และ end (รวมทั้งสองค่า)
// RangeQuery iterates over items where the key is between start and end (inclusive).
// The iteration stops if the provided function f returns false.
// และเรียกใช้ฟังก์ชัน f สำหรับแต่ละคู่ key-value
// การวนลูปจะหยุดลงหากฟังก์ชัน f คืนค่า false
func (sl *SkipList[K, V]) RangeQuery(start, end K, f func(key K, value V) bool) {
	sl.mutex.RLock()
	defer sl.mutex.RUnlock()

	// 1. ค้นหาโหนดเริ่มต้น (โหนดแรกที่มี key >= start)
	// ใช้ตรรกะเดียวกับการค้นหาปกติเพื่อไปยังตำแหน่งที่ถูกต้องในเวลา O(log N)
	current := sl.header
	for i := sl.level; i >= 0; i-- {
		for current.forward[i] != nil && sl.compare(current.forward[i].key, start) < 0 {
			current = current.forward[i]
		}
	}
	// เลื่อนไปยังโหนดแรกที่อาจจะอยู่ในช่วงที่กำหนด
	current = current.forward[0]

	// 2. วนลูปไปข้างหน้าจนกว่า key จะเกินค่า end
	for current != nil && sl.compare(current.key, end) <= 0 {
		// เรียกใช้ callback function และหยุดถ้ามันคืนค่า false
		if !f(current.key, current.value) {
			break
		}
		// ไปยังโหนดถัดไปในชั้นล่างสุด
		current = current.forward[0]
	}
}

// Predecessor ค้นหา key-value คู่ของโหนดที่อยู่ก่อนหน้า (predecessor) ของ key ที่กำหนด
// Predecessor finds the key-value pair of the node that precedes the given key.
// The predecessor is the node with the largest key that is smaller than the target key.
// It returns the node and true if found, otherwise it returns nil and false.
// Predecessor คือโหนดที่มี key มากที่สุดซึ่งน้อยกว่า key ที่กำหนด
// คืนค่าโหนดและ true หากพบ, มิฉะนั้นคืนค่า nil และ false
func (sl *SkipList[K, V]) Predecessor(key K) (INode[K, V], bool) {
	sl.mutex.RLock()
	defer sl.mutex.RUnlock()

	current := sl.header

	// ค้นหาโหนดที่อยู่ก่อนหน้า key ที่กำหนด
	// วิ่งไปข้างหน้าในแต่ละชั้นจนกว่าโหนดถัดไปจะมี key มากกว่าหรือเท่ากับ key ที่ค้นหา
	// หรือเป็น nil
	for i := sl.level; i >= 0; i-- {
		// The key difference for Predecessor is the strict inequality '<'.
		// We stop *before* we reach a node with a key equal to or greater than the target.
		for current.forward[i] != nil && sl.compare(current.forward[i].key, key) < 0 {
			current = current.forward[i]
		}
	}

	// หลังจากลูปสิ้นสุด, 'current' คือโหนดที่มี key น้อยกว่า 'key'
	// และเป็นโหนดที่ใกล้เคียงที่สุด (มากที่สุด) ที่น้อยกว่า 'key'
	// ถ้า 'current' ยังคงเป็น header แสดงว่าไม่มีโหนดข้อมูลใดๆ ที่มี key น้อยกว่า 'key'
	if current != sl.header {
		return current, true
	}

	return nil, false
}

// CountRange นับจำนวนรายการที่ key อยู่ระหว่าง start และ end (รวมทั้งสองค่า)
// CountRange counts the number of items where the key is between start and end (inclusive).
func (sl *SkipList[K, V]) CountRange(start, end K) int {
	sl.mutex.RLock()
	defer sl.mutex.RUnlock()

	// Handle invalid range
	if sl.compare(start, end) > 0 {
		return 0
	}

	count := 0
	current := sl.header

	// 1. ค้นหาโหนดเริ่มต้น (โหนดแรกที่มี key >= start)
	for i := sl.level; i >= 0; i-- {
		for current.forward[i] != nil && sl.compare(current.forward[i].key, start) < 0 {
			current = current.forward[i]
		}
	}
	current = current.forward[0] // Move to the first node that might be in range

	// 2. วนลูปไปข้างหน้าจนกว่า key จะเกินค่า end
	for current != nil && sl.compare(current.key, end) <= 0 {
		count++
		current = current.forward[0]
	}

	return count
}

// Successor ค้นหา key-value คู่ของโหนดที่อยู่ถัดไป (successor) ของ key ที่กำหนด
// Successor finds the key-value pair of the node that succeeds the given key.
// The successor is the node with the smallest key that is larger than the given key.
// It returns the node and true if found, otherwise it returns nil and false.
// Successor คือโหนดที่มี key น้อยที่สุดที่มากกว่า key ที่กำหนด
// คืนค่าโหนดและ true หากพบ, มิฉะนั้นคืนค่า nil และ false
func (sl *SkipList[K, V]) Successor(key K) (INode[K, V], bool) {
	sl.mutex.RLock()
	defer sl.mutex.RUnlock()

	current := sl.header

	// ค้นหาโหนดที่อยู่ก่อนหน้าหรือเท่ากับ key ที่กำหนด
	// วิ่งไปข้างหน้าในแต่ละชั้นจนกว่าโหนดถัดไปจะมี key มากกว่า key ที่ค้นหา
	// หรือเป็น nil
	for i := sl.level; i >= 0; i-- {
		// The key difference for Successor is the non-strict inequality '<='.
		// We advance *past* any node with a key equal to the target.
		for current.forward[i] != nil && sl.compare(current.forward[i].key, key) <= 0 {
			current = current.forward[i]
		}
	}

	// หลังจากลูปสิ้นสุด, 'current' คือโหนดที่มี key น้อยกว่าหรือเท่ากับ 'key'
	// โหนดถัดไปในชั้นล่างสุด (forward[0]) คือ successor ที่เราต้องการ
	if current != nil && current.forward[0] != nil {
		return current.forward[0], true
	}
	return nil, false
}

// Seek finds the first node with a key greater than or equal to the given key.
// It returns the node and true if such a node is found, otherwise it returns nil and false.
// Seek ค้นหาโหนดแรกที่มี key เท่ากับหรือมากกว่า key ที่กำหนด
// คืนค่าโหนดและ true หากพบ, มิฉะนั้นคืนค่า nil และ false
func (sl *SkipList[K, V]) Seek(key K) (INode[K, V], bool) {
	sl.mutex.RLock()
	defer sl.mutex.RUnlock()

	current := sl.header

	// Find the node preceding the target position.
	for i := sl.level; i >= 0; i-- {
		for current.forward[i] != nil && sl.compare(current.forward[i].key, key) < 0 {
			current = current.forward[i]
		}
	}

	// The next node is the first one with a key >= key.
	current = current.forward[0]

	if current != nil {
		return current, true
	}

	return nil, false
}

// PopMin ดึง key-value คู่ที่มี key น้อยที่สุดออกจาก skiplist และลบโหนดนั้นออก
// PopMin removes and returns the smallest key-value pair from the skiplist.
// It returns a node containing the popped data and true if an item was popped,
// otherwise it returns nil and false.
// คืนค่าโหนดที่เก็บข้อมูลที่ถูกดึงออกและ true หากมีรายการ, มิฉะนั้นคืนค่า nil และ false
func (sl *SkipList[K, V]) PopMin() (INode[K, V], bool) {
	sl.mutex.Lock() // ใช้ Lock เพราะมีการแก้ไขโครงสร้าง
	defer sl.mutex.Unlock()

	if sl.length == 0 {
		return nil, false
	}

	// โหนดที่มี key น้อยที่สุดคือโหนดแรกในชั้น 0
	// ดึง Key และ Value ออกมาก่อนที่โหนดจะถูกเคลียร์โดย deleteNode
	nodeToRemove := sl.header.forward[0]
	poppedKey := nodeToRemove.key
	poppedValue := nodeToRemove.value

	// สำหรับ PopMin, 'update' path คือ header ในทุกชั้น
	update := sl.updateCache
	for i := 0; i <= sl.level; i++ {
		update[i] = sl.header
	}

	sl.deleteNode(nodeToRemove, update)
	return &node[K, V]{key: poppedKey, value: poppedValue}, true
}

// PopMax ดึง key-value คู่ที่มี key มากที่สุดออกจาก skiplist และลบโหนดนั้นออก
// PopMax removes and returns the largest key-value pair from the skiplist.
// It returns a node containing the popped data and true if an item was popped,
// otherwise it returns nil and false.
// คืนค่าโหนดที่เก็บข้อมูลที่ถูกดึงออกและ true หากมีรายการ, มิฉะนั้นคืนค่า nil และ false
func (sl *SkipList[K, V]) PopMax() (INode[K, V], bool) {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	if sl.length == 0 {
		return nil, false
	}

	// --- ขั้นตอนที่ 1: ค้นหาโหนดสุดท้าย (Max Node) ---
	// ใช้ตรรกะเดียวกับฟังก์ชัน Max() เพื่อหาโหนดที่อยู่ท้ายสุดของ list
	// ซึ่งเป็นโหนดที่เราต้องการจะลบ
	nodeToRemove := sl.header
	for i := sl.level; i >= 0; i-- {
		for nodeToRemove.forward[i] != nil {
			nodeToRemove = nodeToRemove.forward[i]
		}
	}

	// --- ขั้นตอนที่ 2: ค้นหา update path สำหรับโหนดที่จะลบ ---
	// เมื่อเราได้ nodeToRemove แล้ว เราจะใช้ key ของมันเพื่อค้นหา update path
	// โดยใช้ตรรกะการค้นหาแบบมาตรฐานเหมือนในฟังก์ชัน Delete
	// วิธีนี้ทำให้โค้ดมีความสอดคล้องและเข้าใจง่ายขึ้น
	update := sl.updateCache
	current := sl.header
	keyToRemove := nodeToRemove.key
	for i := sl.level; i >= 0; i-- {
		for current.forward[i] != nil && sl.compare(current.forward[i].key, keyToRemove) < 0 {
			current = current.forward[i]
		}
		update[i] = current
	}

	// --- ขั้นตอนที่ 3: ดึงข้อมูลและลบโหนด ---
	// ดึง Key และ Value ออกมาก่อนที่โหนดจะถูกเคลียร์โดย deleteNode
	poppedKey := nodeToRemove.key     // ใช้ข้อมูลจาก nodeToRemove ที่หาไว้
	poppedValue := nodeToRemove.value // ใช้ข้อมูลจาก nodeToRemove ที่หาไว้

	sl.deleteNode(nodeToRemove, update)
	return &node[K, V]{key: poppedKey, value: poppedValue}, true
}
