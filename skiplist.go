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
	header               *node[K, V]         // โหนดเริ่มต้น (sentinel node)
	level                int                 // ชั้นสูงสุดที่มีอยู่ในปัจจุบัน
	length               int                 // จำนวนรายการทั้งหมดใน skiplist
	rand                 *rand.Rand          // ตัวสร้างเลขสุ่มสำหรับกำหนดชั้น
	mutex                sync.RWMutex        // Mutex สำหรับการทำงานแบบ concurrent-safe
	updateCacheRanks     []int               // แคชสำหรับ rank ที่ใช้ใน Insert
	updateCache          []INode[K, V]       // แคชสำหรับ update path
	allocator            nodeAllocator[K, V] // Abstraction สำหรับการจัดสรรหน่วยความจำ
	arenaInitialSize     int                 // ขนาดเริ่มต้นของ Arena (ถ้าใช้)
	arenaGrowthFactor    float64             // สัดส่วนการขยาย Arena (ถ้าใช้)
	arenaGrowthBytes     int                 // ขนาด byte คงที่ในการขยาย Arena (ถ้าใช้)
	arenaGrowthThreshold float64             // Threshold สำหรับการขยาย Arena ล่วงหน้า (ถ้าใช้)
	compare              Comparator[K]       // ฟังก์ชันสำหรับเปรียบเทียบ key
}

// Option is a function that configures a SkipList.
// Option คือฟังก์ชันสำหรับกำหนดค่าของ SkipList
type Option[K any, V any] func(*SkipList[K, V])

// WithArena configures the SkipList to use a memory arena of a given size in bytes.
// This sets the initial size of the arena. The arena can grow automatically if needed.
// WithArena กำหนดให้ SkipList ใช้ memory arena ตามขนาดที่ระบุ (เป็น byte)
// เป็นการกำหนดขนาดเริ่มต้น และ arena สามารถขยายขนาดได้เองเมื่อจำเป็น
func WithArena[K any, V any](sizeInBytes int) Option[K, V] {
	return func(sl *SkipList[K, V]) {
		if sizeInBytes > 0 {
			sl.arenaInitialSize = sizeInBytes
		}
	}
}

// WithArenaGrowthFactor configures the arena to grow by a factor of the previous chunk's size.
// For example, a factor of 2.0 means each new chunk will be twice as large as the last.
// This option is only effective when used with WithArena.
func WithArenaGrowthFactor[K any, V any](factor float64) Option[K, V] {
	return func(sl *SkipList[K, V]) {
		if factor > 1.0 {
			sl.arenaGrowthFactor = factor
		}
	}
}

// WithArenaGrowthBytes configures the arena to grow by a fixed number of bytes.
// This option is only effective when used with WithArena.
func WithArenaGrowthBytes[K any, V any](bytes int) Option[K, V] {
	return func(sl *SkipList[K, V]) {
		if bytes > 0 {
			sl.arenaGrowthBytes = bytes
		}
	}
}

// WithArenaGrowthThreshold configures the arena's proactive growth threshold (e.g., 0.9 for 90%).
// If an allocation would cause a chunk's usage to exceed this threshold, the arena will
// grow preemptively. This option is only effective when used with WithArena.
func WithArenaGrowthThreshold[K any, V any](threshold float64) Option[K, V] {
	return func(sl *SkipList[K, V]) {
		// The arena itself validates the range, but we can check here too.
		if threshold > 0.0 && threshold < 1.0 {
			sl.arenaGrowthThreshold = threshold
		}
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
		span:    make([]int, MaxLevel),
	}

	// ใช้ PCG (Permuted Congruential Generator) ซึ่งเป็น default ใน Go 1.22+
	// เพื่อประสิทธิภาพที่ดีกว่า
	source := rand.NewPCG(rand.Uint64(), rand.Uint64())

	sl := &SkipList[K, V]{
		header:           header,
		level:            0, // เริ่มต้นที่ชั้น 0
		length:           0,
		rand:             rand.New(source),
		updateCacheRanks: make([]int, MaxLevel),
		updateCache:      make([]INode[K, V], MaxLevel),
		allocator:        newPoolAllocator[K, V](), // Default to sync.Pool
		compare:          compare,
	}

	// Apply any custom options provided by the user
	for _, opt := range opts {
		opt(sl)
	}

	// After processing options, create the arena if requested.
	if sl.arenaInitialSize > 0 {
		var arenaOpts []ArenaOption
		if sl.arenaGrowthBytes > 0 {
			arenaOpts = append(arenaOpts, WithGrowthBytes(sl.arenaGrowthBytes))
		}
		if sl.arenaGrowthFactor > 1.0 {
			arenaOpts = append(arenaOpts, WithGrowthFactor(sl.arenaGrowthFactor))
		}
		if sl.arenaGrowthThreshold > 0.0 {
			arenaOpts = append(arenaOpts, WithGrowthThreshold(sl.arenaGrowthThreshold))
		}
		sl.allocator = newArenaAllocator[K, V](sl.arenaInitialSize, arenaOpts...)
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
	ranks := sl.updateCacheRanks
	current := sl.header

	// ค้นหาตำแหน่งที่จะเพิ่มโหนดใหม่ พร้อมทั้งบันทึกโหนดที่จะต้องอัปเดต
	// และคำนวณ rank ไปพร้อมกัน
	for i := sl.level; i >= 0; i-- {
		// rank ที่ชั้น i คือ rank ที่คำนวณได้จากชั้น i+1
		if i == sl.level {
			ranks[i] = 0
		} else {
			ranks[i] = ranks[i+1]
		}

		for current.forward[i] != nil && sl.compare(current.forward[i].key, key) < 0 {
			ranks[i] += current.span[i]
			current = current.forward[i]
		}
		update[i] = current
	}

	// rank ของโหนดก่อนหน้าคือ ranks[0]
	// rank ของโหนดใหม่ (0-based) คือ ranks[0]

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
			// สำหรับชั้นใหม่ๆ, update path จะเริ่มจาก header
			update[i] = sl.header
			// rank ที่ชั้นเหล่านี้จะเริ่มต้นที่ 0
			ranks[i] = 0
			// span ของ header ในชั้นใหม่นี้จะต้องครอบคลุมโหนดทั้งหมดที่มีอยู่
			// เพราะ pointer ของมันจะชี้ไปที่ nil (ก่อนที่จะถูกเชื่อมกับโหนดใหม่)
			// ดังนั้น span ของมันคือจำนวนโหนดทั้งหมดใน list
			sl.header.span[i] = sl.length
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
		newNode.span = make([]int, newLevel)
	} else {
		newNode.forward = newNode.forward[:newLevel]
		newNode.span = newNode.span[:newLevel]
	}

	newNode.key = key
	newNode.value = value

	// เชื่อมโหนดใหม่เข้ากับ skiplist ในแต่ละชั้น
	// พร้อมทั้งอัปเดตค่า span
	for i := 0; i < newLevel; i++ {
		cupdate, ok := update[i].(*node[K, V])
		if !ok {
			continue
		}
		// เชื่อม forward pointer
		newNode.forward[i] = cupdate.forward[i]
		cupdate.forward[i] = newNode

		// อัปเดต span
		// newSpan คือระยะห่างจาก cupdate ไปยัง newNode
		newSpan := (ranks[0] - ranks[i]) + 1
		newNode.span[i] = cupdate.span[i] - (newSpan - 1)
		cupdate.span[i] = newSpan
	}

	// สำหรับชั้นที่สูงกว่า newLevel, เราแค่เพิ่ม span ของโหนดใน update path
	// เพราะมีโหนดใหม่เพิ่มเข้ามาในเส้นทางนั้น
	for i := newLevel; i <= sl.level; i++ {
		update[i].(*node[K, V]).span[i]++
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
		cupdate, _ := update[i].(*node[K, V])
		if cupdate.forward[i] == cnodeRemove {
			// ถ้าโหนดใน update path ชี้ไปยังโหนดที่จะลบโดยตรง
			// ให้รวม span ของโหนดที่ถูกลบเข้ามา แล้วลบออก 1 (ตัวโหนดเอง)
			cupdate.span[i] += cnodeRemove.span[i] - 1
			cupdate.forward[i] = cnodeRemove.forward[i]
		} else {
			// ถ้าโหนดใน update path อยู่ในชั้นที่สูงกว่าโหนดที่จะลบ
			// และไม่ได้ชี้ไปยังโหนดนั้นโดยตรง (ทางเดินมัน "ข้าม" โหนดที่จะลบไป)
			// เราแค่ลด span ลง 1 เพราะมีโหนดหายไปจาก list
			if cupdate.forward[i] != nil {
				cupdate.span[i]--
			}
		}
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

	// Create an "unsafe" iterator that doesn't perform its own locking,
	// because we're already holding a read lock.
	it := sl.NewIterator(withUnsafe[K, V]())
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

// findGreaterOrEqual finds the first node with a key >= the given key.
// It returns nil if no such node is found.
// The caller must hold a lock.
// findGreaterOrEqual ค้นหาโหนดแรกที่มี key มากกว่าหรือเท่ากับ key ที่กำหนด
// คืนค่า nil หากไม่พบโหนดดังกล่าว
// ผู้เรียกต้องถือ lock อยู่แล้ว
func (sl *SkipList[K, V]) findGreaterOrEqual(key K) *node[K, V] {
	current := sl.header
	for i := sl.level; i >= 0; i-- {
		for current.forward[i] != nil && sl.compare(current.forward[i].key, key) < 0 {
			current = current.forward[i]
		}
	}
	// The next node is the first one with a key >= key.
	return current.forward[0]
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
	current := sl.findGreaterOrEqual(start)

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
	// 1. ค้นหาโหนดเริ่มต้น (โหนดแรกที่มี key >= start)
	current := sl.findGreaterOrEqual(start)

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

	node := sl.findGreaterOrEqual(key)

	if node != nil {
		return node, true
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

// Rank returns the 0-based rank of the given key.
// The rank is the number of elements with keys strictly smaller than the given key.
// If the key is not in the list, it returns the rank it would have if it were inserted.
// The complexity is O(log n).
// Rank คืนค่าอันดับ (0-based) ของ key ที่กำหนด
// อันดับคือจำนวนรายการที่มี key น้อยกว่า key ที่ระบุ
// หากไม่พบ key จะคืนค่าอันดับที่ควรจะเป็นหากมีการเพิ่ม key นั้นเข้าไป
// มีความซับซ้อน O(log n)
func (sl *SkipList[K, V]) Rank(key K) int {
	sl.mutex.RLock()
	defer sl.mutex.RUnlock()

	rank := 0
	current := sl.header

	for i := sl.level; i >= 0; i-- {
		for current.forward[i] != nil && sl.compare(current.forward[i].key, key) < 0 {
			rank += current.span[i]
			current = current.forward[i]
		}
	}
	return rank
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

	// --- ขั้นตอนที่ 1: ค้นหาโหนดสุดท้าย (Max Node) เพื่อหา key ---
	// การทำ 2-pass (หา key ก่อน แล้วค่อยหา update path) อาจมีประสิทธิภาพด้อยกว่าเล็กน้อย
	// แต่ช่วยให้ตรรกะการลบสอดคล้องกับฟังก์ชัน Delete และลดความซับซ้อน
	// ทำให้การอัปเดต span ถูกต้องและง่ายต่อการตรวจสอบ
	lastNode := sl.header
	for i := sl.level; i >= 0; i-- {
		for lastNode.forward[i] != nil {
			lastNode = lastNode.forward[i]
		}
	}
	keyToRemove := lastNode.key

	// --- ขั้นตอนที่ 2: ค้นหา update path สำหรับ key ที่จะลบ (เหมือนในฟังก์ชัน Delete) ---
	update := sl.updateCache
	current := sl.header
	for i := sl.level; i >= 0; i-- {
		for current.forward[i] != nil && sl.compare(current.forward[i].key, keyToRemove) < 0 {
			current = current.forward[i]
		}
		update[i] = current
	}

	// --- ขั้นตอนที่ 3: ดึงข้อมูลและลบโหนด ---
	nodeToRemove := current.forward[0]

	// ดึง Key และ Value ออกมาก่อนที่โหนดจะถูกเคลียร์โดย deleteNode
	poppedKey := nodeToRemove.key
	poppedValue := nodeToRemove.value

	sl.deleteNode(nodeToRemove, update)
	return &node[K, V]{key: poppedKey, value: poppedValue}, true
}

// GetByRank returns the node at the given 0-based rank.
// If the rank is out of bounds (rank < 0 or rank >= sl.Len()), it returns nil and false.
// The complexity is O(log n).
// GetByRank คืนค่าโหนด ณ อันดับที่กำหนด (0-based)
// หากอันดับอยู่นอกขอบเขต (น้อยกว่า 0 หรือมากกว่าหรือเท่ากับ Len()) จะคืนค่า nil และ false
// มีความซับซ้อน O(log n)
func (sl *SkipList[K, V]) GetByRank(rank int) (INode[K, V], bool) {
	sl.mutex.RLock()
	defer sl.mutex.RUnlock()

	if rank < 0 || rank >= sl.length {
		return nil, false
	}

	var traversed int = -1 // Header is at rank -1
	current := sl.header

	for i := sl.level; i >= 0; i-- {
		for current.forward[i] != nil && (traversed+current.span[i]) <= rank {
			traversed += current.span[i]
			current = current.forward[i]
		}
	}
	return current, true
}
