package skiplist

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"sync"
	"testing"
)

// testSetup defines a configuration for creating a SkipList for testing.
type testSetup[K cmp.Ordered, V any] struct {
	name        string
	constructor func(compare Comparator[K], opts ...Option[K, V]) *SkipList[K, V]
}

// getTestSetups returns a slice of test setups for a given key and value type.
// This allows all tests to be run against both the pool-based and arena-based allocators.
func getTestSetups[K cmp.Ordered, V any]() []testSetup[K, V] {
	return []testSetup[K, V]{
		{
			name: "WithPool",
			constructor: func(compare Comparator[K], opts ...Option[K, V]) *SkipList[K, V] {
				if compare == nil {
					return New[K, V](opts...)
				}
				return NewWithComparator[K, V](compare, opts...)
			},
		},
		{
			name: "WithArena",
			constructor: func(compare Comparator[K], opts ...Option[K, V]) *SkipList[K, V] {
				arenaSize := 1024 * 1024 // 1MB Arena for tests
				allOpts := append([]Option[K, V]{WithArena[K, V](arenaSize)}, opts...)
				if compare == nil {
					return New[K, V](allOpts...)
				}
				return NewWithComparator[K, V](compare, allOpts...)
			},
		},
	}
}

func TestSkipList(t *testing.T) {
	for _, setup := range getTestSetups[int, string]() {
		t.Run(setup.name, func(t *testing.T) {
			// --- ทดสอบ Min/Max กับ list ว่าง ---
			emptySl := setup.constructor(nil)
			if _, ok := emptySl.Min(); ok {
				t.Error("Min() on empty list should return ok=false")
			}

			// สร้าง skiplist ใหม่สำหรับ <int, string>
			sl := setup.constructor(func(a, b int) int { return a - b }) // Test custom comparator

			// --- ทดสอบ Insert และ Len (ใช้ intComparator ที่ระบุ) ---
			sl.Insert(10, "ten")
			sl.Insert(5, "five")
			sl.Insert(20, "twenty")
			sl.Insert(15, "fifteen")

			if sl.Len() != 4 {
				t.Errorf("Expected length 4, but got %d", sl.Len())
			}

			// --- ทดสอบ Search ---
			node, ok := sl.Search(15) // Search now returns *Node[K,V], bool
			if !ok || node.Value() != "fifteen" {
				t.Errorf("Search(15): expected 'fifteen', got '%s'", node.Value())
			}

			_, ok = sl.Search(99)
			if ok {
				t.Error("Search(99): expected not found, but was found")
			}

			// --- ทดสอบการอัปเดตค่า ---
			sl.Insert(10, "TEN_UPDATED")
			node, ok = sl.Search(10)
			if !ok || node.Value() != "TEN_UPDATED" { // 'node' is from Search(10)
				t.Errorf("Update(10): expected 'TEN_UPDATED', got '%s'", node.Value()) // Corrected: use node.Value()
			}
			if sl.Len() != 4 {
				t.Errorf("Expected length 4 after update, but got %d", sl.Len())
			}

			// --- ทดสอบ Min และ Max ---
			minNode, ok := sl.Min()
			if !ok || minNode.Key() != 5 || minNode.Value() != "five" {
				t.Errorf("Min(): expected (5, 'five'), got (%v, '%v')", minNode.Key(), minNode.Value())
			}

			maxNode, ok := sl.Max()
			if !ok || maxNode.Key() != 20 || maxNode.Value() != "twenty" {
				t.Errorf("Max(): expected (20, 'twenty'), got (%v, '%v')", maxNode.Key(), maxNode.Value())
			}

			if _, ok := emptySl.Max(); ok {
				t.Error("Max() on empty list should return ok=false")
			}

			// --- ทดสอบ Delete ---
			deleted := sl.Delete(5)
			if !deleted {
				t.Error("Delete(5): expected to delete, but key not found")
			}
			if sl.Len() != 3 {
				t.Errorf("Expected length 3 after delete, but got %d", sl.Len())
			}
			_, ok = sl.Search(5)
			if ok {
				t.Error("Search(5): expected not found after delete, but was found")
			}
			// ตรวจสอบ Min อีกครั้งหลังลบ
			minNode, ok = sl.Min()
			if !ok || minNode.Key() != 10 || minNode.Value() != "TEN_UPDATED" {
				t.Errorf("Min() after delete: expected (10, 'TEN_UPDATED'), got (%v, '%v')", minNode.Key(), minNode.Value())
			}

			deleted = sl.Delete(100)
			if deleted {
				t.Error("Delete(100): expected not to delete non-existent key, but it did")
			}

			// --- ทดสอบ Range ---
			fmt.Println("--- Items in SkipList (sorted by key) ---")
			var keys []int
			sl.Range(func(key int, value string) bool {
				fmt.Printf("Key: %d, Value: %s\n", key, value) // Corrected: use 'key' and 'value' from Range callback
				keys = append(keys, key)
				return true // คืนค่า true เพื่อวนลูปต่อไป
			})

			expectedKeys := []int{10, 15, 20}
			if len(keys) != len(expectedKeys) {
				t.Errorf("Range: expected %d keys, got %d", len(expectedKeys), len(keys))
			}
			for i, k := range keys {
				if k != expectedKeys[i] {
					t.Errorf("Range: expected key %d at index %d, got %d", expectedKeys[i], i, k)
				}
			}

			// --- ทดสอบ Range ด้วยเงื่อนไขหยุดกลางคัน ---
			t.Run("Range with break condition", func(t *testing.T) {
				var collectedKeys []int
				// จะหยุดเมื่อ key มีค่ามากกว่า 10
				sl.Range(func(key int, value string) bool {
					collectedKeys = append(collectedKeys, key)
					return key <= 10 // คืนค่า false เมื่อ key > 10
				})

				// คาดว่าจะเก็บได้ [10, 15] เพราะ callback จะทำงานกับ key 15 ก่อนที่จะคืนค่า false และหยุดลูป
				expectedCollectedKeys := []int{10, 15}
				if len(collectedKeys) != len(expectedCollectedKeys) {
					t.Errorf("Range with break: Expected %d keys, got %d. Keys: %v", len(expectedCollectedKeys), len(collectedKeys), collectedKeys)
				}
				for i, k := range collectedKeys {
					if k != expectedCollectedKeys[i] {
						t.Errorf("Range with break: Expected key %d at index %d, got %d", expectedCollectedKeys[i], i, k)
					}
				}
			})
		})
	}
}

func TestSkipList_Clear(t *testing.T) {
	for _, setup := range getTestSetups[int, string]() {
		t.Run(setup.name, func(t *testing.T) {
			sl := setup.constructor(nil)
			sl.Insert(10, "ten")
			sl.Insert(20, "twenty")
			sl.Insert(30, "thirty")

			if sl.Len() != 3 {
				t.Fatalf("Expected length 3 before Clear, got %d", sl.Len())
			}

			// Clear the list
			sl.Clear()

			// Check if the list is empty
			if sl.Len() != 0 {
				t.Errorf("Expected length 0 after Clear, got %d", sl.Len())
			}

			if _, ok := sl.Search(20); ok {
				t.Error("Search(20) should fail after Clear, but it succeeded")
			}

			if _, ok := sl.Min(); ok {
				t.Error("Min() on cleared list should return ok=false")
			}

			if _, ok := sl.Max(); ok {
				t.Error("Max() on cleared list should return ok=false")
			}

			// Test if the list is usable after clearing
			sl.Insert(100, "one hundred")
			sl.Insert(50, "fifty")

			if sl.Len() != 2 {
				t.Errorf("Expected length 2 after re-inserting, got %d", sl.Len())
			}

			node, ok := sl.Search(100)
			if !ok || node.Value() != "one hundred" {
				t.Errorf("Search(100) after Clear and re-insert failed")
			}

			minNode, ok := sl.Min()
			if !ok || minNode.Key() != 50 {
				t.Errorf("Min() after Clear and re-insert failed, expected 50, got %v", minNode.Key())
			}
		})
	}
}

func TestSkipListConcurrent(t *testing.T) {
	for _, setup := range getTestSetups[int, int]() {
		t.Run(setup.name, func(t *testing.T) {
			sl := setup.constructor(nil)
			wg := sync.WaitGroup{}

			numGoroutines := 100
			itemsPerGoroutine := 100
			totalItems := numGoroutines * itemsPerGoroutine

			// --- ทดสอบการ Insert พร้อมกัน ---
			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				go func(g_id int) {
					defer wg.Done()
					for j := 0; j < itemsPerGoroutine; j++ {
						key := g_id*itemsPerGoroutine + j
						sl.Insert(key, key)
					}
				}(i)
			}
			wg.Wait()

			if sl.Len() != totalItems {
				t.Fatalf("Expected length %d after concurrent inserts, got %d", totalItems, sl.Len())
			}

			// --- ทดสอบการ Search และ Delete พร้อมกัน ---
			// Goroutine คู่จะลบ, Goroutine คี่จะอ่าน
			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				go func(g_id int) {
					defer wg.Done()
					for j := 0; j < itemsPerGoroutine; j++ {
						key := g_id*itemsPerGoroutine + j
						if g_id%2 == 0 {
							sl.Delete(key)
						} else {
							sl.Search(key) // แค่ค้นหาเพื่อทดสอบ race condition
						}
					}
				}(i)
			}
			wg.Wait()

			expectedLen := totalItems / 2
			if sl.Len() != expectedLen {
				t.Fatalf("Expected length %d after concurrent deletes, got %d", expectedLen, sl.Len())
			}
		})
	}
}

func TestSkipList_RangeQuery(t *testing.T) {
	for _, setup := range getTestSetups[int, string]() {
		t.Run(setup.name, func(t *testing.T) {
			sl := setup.constructor(nil)
			sl.Insert(10, "ten")
			sl.Insert(20, "twenty")
			sl.Insert(30, "thirty")
			sl.Insert(40, "forty")
			sl.Insert(50, "fifty")

			t.Run("ค้นหาช่วงข้อมูลกลางๆ", func(t *testing.T) {
				var keys []int
				var values []string
				sl.RangeQuery(15, 45, func(key int, value string) bool {
					keys = append(keys, key)
					values = append(values, value)
					return true
				})

				expectedKeys := []int{20, 30, 40}
				expectedValues := []string{"twenty", "thirty", "forty"}

				if len(keys) != len(expectedKeys) {
					t.Fatalf("Expected %d keys, got %d. Keys: %v", len(expectedKeys), len(keys), keys)
				}

				for i, k := range keys {
					if k != expectedKeys[i] || values[i] != expectedValues[i] {
						t.Errorf("Expected (%d, %s) at index %d, got (%d, %s)",
							expectedKeys[i], expectedValues[i], i, k, values[i])
					}
				}
			})

			t.Run("ค้นหาช่วงที่ไม่มีข้อมูล", func(t *testing.T) {
				var keys []int
				sl.RangeQuery(21, 29, func(key int, value string) bool {
					keys = append(keys, key)
					return true
				})
				if len(keys) != 0 {
					t.Fatalf("Expected 0 keys, got %d", len(keys))
				}
			})

			t.Run("หยุดการทำงานกลางคัน", func(t *testing.T) {
				var keys []int
				sl.RangeQuery(10, 50, func(key int, value string) bool {
					keys = append(keys, key)
					return key < 30 // หยุดเมื่อ key มีค่าเป็น 30
				})

				expectedKeys := []int{10, 20, 30}
				if len(keys) != len(expectedKeys) {
					t.Fatalf("Expected %d keys, got %d. Keys: %v", len(expectedKeys), len(keys), keys)
				}
				for i, k := range keys {
					if k != expectedKeys[i] {
						t.Errorf("Expected key %d at index %d, got %d", expectedKeys[i], i, k)
					}
				}
			})

			t.Run("ช่วงข้อมูลไม่ถูกต้อง (start > end)", func(t *testing.T) {
				var keys []int
				sl.RangeQuery(40, 20, func(key int, value string) bool {
					keys = append(keys, key)
					return true
				})
				if len(keys) != 0 {
					t.Fatalf("Expected 0 keys for invalid range, got %d", len(keys))
				}
			})
		})
	}
}

func TestSkipList_Predecessor(t *testing.T) {
	for _, setup := range getTestSetups[int, string]() {
		t.Run(setup.name, func(t *testing.T) {
			sl := setup.constructor(nil)

			// Test on empty list
			_, ok := sl.Predecessor(10)
			if ok {
				t.Error("Predecessor on empty list should return false")
			}

			sl.Insert(10, "ten")
			sl.Insert(20, "twenty")
			sl.Insert(30, "thirty")
			sl.Insert(40, "forty")
			sl.Insert(50, "fifty")

			// Test key that exists
			node, ok := sl.Predecessor(30)
			if !ok || node.Key() != 20 || node.Value() != "twenty" {
				t.Errorf("Predecessor(30): Expected (20, 'twenty'), got (%v, '%v', %v)", node.Key(), node.Value(), ok)
			}

			// Test key that does not exist, but has a predecessor
			node, ok = sl.Predecessor(25)
			if !ok || node.Key() != 20 || node.Value() != "twenty" {
				t.Errorf("Predecessor(25): Expected (20, 'twenty'), got (%v, '%v', %v)", node.Key(), node.Value(), ok)
			}

			// Test key that is the smallest in the list
			node, ok = sl.Predecessor(10)
			if ok { // No predecessor for the smallest element
				t.Errorf("Predecessor(10): Expected no predecessor, got (%v, '%v', %v)", node.Key(), node.Value(), ok)
			}

			// Test key smaller than the smallest element
			node, ok = sl.Predecessor(5)
			if ok { // No predecessor for a key smaller than the smallest element
				t.Errorf("Predecessor(5): Expected no predecessor, got (%v, '%v', %v)", node.Key(), node.Value(), ok)
			}

			// Test key that is the largest in the list
			node, ok = sl.Predecessor(50)
			if !ok || node.Key() != 40 || node.Value() != "forty" {
				t.Errorf("Predecessor(50): Expected (40, 'forty'), got (%v, '%v', %v)", node.Key(), node.Value(), ok)
			}

			// Test key larger than the largest element
			node, ok = sl.Predecessor(55)
			if !ok || node.Key() != 50 || node.Value() != "fifty" {
				t.Errorf("Predecessor(55): Expected (50, 'fifty'), got (%v, '%v', %v)", node.Key(), node.Value(), ok)
			}

			// Test after deletion
			sl.Delete(30)
			node, ok = sl.Predecessor(40)
			if !ok || node.Key() != 20 || node.Value() != "twenty" {
				t.Errorf("Predecessor(40) after deleting 30: Expected (20, 'twenty'), got (%v, '%v', %v)", node.Key(), node.Value(), ok)
			}
			node, ok = sl.Predecessor(35) // Key that was between 30 and 40
			if !ok || node.Key() != 20 || node.Value() != "twenty" {
				t.Errorf("Predecessor(35) after deleting 30: Expected (20, 'twenty'), got (%v, '%v', %v)", node.Key(), node.Value(), ok)
			}
		})
	}
}

func TestSkipList_Successor(t *testing.T) {
	for _, setup := range getTestSetups[int, string]() {
		t.Run(setup.name, func(t *testing.T) {
			sl := setup.constructor(nil)

			// Test on empty list
			_, ok := sl.Successor(10)
			if ok {
				t.Error("Successor on empty list should return false")
			}

			sl.Insert(10, "ten")
			sl.Insert(20, "twenty")
			sl.Insert(30, "thirty")
			sl.Insert(40, "forty")
			sl.Insert(50, "fifty")

			// Test key that exists
			node, ok := sl.Successor(30)
			if !ok || node.Key() != 40 || node.Value() != "forty" {
				t.Errorf("Successor(30): Expected (40, 'forty'), got (%v, '%v', %v)", node.Key(), node.Value(), ok)
			}

			// Test key that does not exist, but has a successor
			node, ok = sl.Successor(25)
			if !ok || node.Key() != 30 || node.Value() != "thirty" {
				t.Errorf("Successor(25): Expected (30, 'thirty'), got (%v, '%v', %v)", node.Key(), node.Value(), ok)
			}

			// Test key that is the largest in the list
			node, ok = sl.Successor(50)
			if ok { // No successor for the largest element
				t.Errorf("Successor(50): Expected no successor, got (%v, '%v', %v)", node.Key(), node.Value(), ok)
			}

			// Test key larger than the largest element
			node, ok = sl.Successor(55)
			if ok { // No successor for a key larger than the largest element
				t.Errorf("Successor(55): Expected no successor, got (%v, '%v', %v)", node.Key(), node.Value(), ok)
			}

			// Test key that is the smallest in the list
			node, ok = sl.Successor(10)
			if !ok || node.Key() != 20 || node.Value() != "twenty" {
				t.Errorf("Successor(10): Expected (20, 'twenty'), got (%v, '%v', %v)", node.Key(), node.Value(), ok)
			}

			// Test key smaller than the smallest element
			node, ok = sl.Successor(5)
			if !ok || node.Key() != 10 || node.Value() != "ten" {
				t.Errorf("Successor(5): Expected (10, 'ten'), got (%v, '%v', %v)", node.Key(), node.Value(), ok)
			}

			// Test after deletion
			sl.Delete(30)
			node, ok = sl.Successor(20)
			if !ok || node.Key() != 40 || node.Value() != "forty" {
				t.Errorf("Successor(20) after deleting 30: Expected (40, 'forty'), got (%v, '%v', %v)", node.Key(), node.Value(), ok)
			}
			node, ok = sl.Successor(35) // Key that was between 30 and 40
			if !ok || node.Key() != 40 || node.Value() != "forty" {
				t.Errorf("Successor(35) after deleting 30: Expected (40, 'forty'), got (%v, '%v', %v)", node.Key(), node.Value(), ok)
			}
		})
	}
}

func TestSkipList_Seek(t *testing.T) {
	for _, setup := range getTestSetups[int, string]() {
		t.Run(setup.name, func(t *testing.T) {
			sl := setup.constructor(nil)

			// Test on empty list
			_, ok := sl.Seek(10)
			if ok {
				t.Error("Seek on empty list should return false")
			}

			sl.Insert(10, "ten")
			sl.Insert(20, "twenty")
			sl.Insert(40, "forty")
			sl.Insert(50, "fifty")

			// Test seeking to an existing key
			node, ok := sl.Seek(20)
			if !ok || node.Key() != 20 || node.Value() != "twenty" {
				t.Errorf("Seek(20): Expected (20, 'twenty'), but got (%v, '%v')", node.Key(), node.Value())
			}

			// Test seeking to a non-existing key (between two keys)
			node, ok = sl.Seek(25)
			if !ok || node.Key() != 40 || node.Value() != "forty" {
				t.Errorf("Seek(25): Expected (40, 'forty'), but got (%v, '%v')", node.Key(), node.Value())
			}

			// Test seeking to the first key
			node, ok = sl.Seek(10)
			if !ok || node.Key() != 10 || node.Value() != "ten" {
				t.Errorf("Seek(10): Expected (10, 'ten'), but got (%v, '%v')", node.Key(), node.Value())
			}

			// Test seeking to a key smaller than the minimum key
			node, ok = sl.Seek(5)
			if !ok || node.Key() != 10 || node.Value() != "ten" {
				t.Errorf("Seek(5): Expected (10, 'ten'), but got (%v, '%v')", node.Key(), node.Value())
			}

			// Test seeking to the last key
			node, ok = sl.Seek(50)
			if !ok || node.Key() != 50 || node.Value() != "fifty" {
				t.Errorf("Seek(50): Expected (50, 'fifty'), but got (%v, '%v')", node.Key(), node.Value())
			}

			// Test seeking to a key larger than the maximum key
			_, ok = sl.Seek(55)
			if ok {
				t.Errorf("Seek(55): Expected no node, but found one with key %v", node.Key())
			}

			// Test after deletion
			sl.Delete(20)
			node, ok = sl.Seek(15)
			if !ok || node.Key() != 40 || node.Value() != "forty" {
				t.Errorf("Seek(15) after deleting 20: Expected (40, 'forty'), but got (%v, '%v')", node.Key(), node.Value())
			}
		})
	}
}

// mockRandSource คือ random source ปลอมที่คืนค่าตัวเลขตามลำดับที่กำหนดไว้ล่วงหน้า
// เพื่อใช้ในการทดสอบที่ต้องการผลลัพธ์ที่แน่นอน
type mockRandSource struct {
	nums []uint64
	pos  int
}

func (m *mockRandSource) Uint64() uint64 {
	if m.pos >= len(m.nums) {
		// คืนค่าที่ x&3 != 0 โดยปริยาย เพื่อให้การสร้าง level หยุดลง
		return 1
	}
	num := m.nums[m.pos]
	m.pos++
	return num
}

// TestSkipList_Insert_NodeReuse tests a specific implementation detail of the pool-based allocator.
// It verifies that a node with a large `forward` slice, when returned to the pool, can be
// reused for a new node that requires a smaller `forward` slice, avoiding a new allocation for the slice.
func TestSkipList_Insert_NodeReuse(t *testing.T) {
	t.Run("Test forward slice reuse from pool", func(t *testing.T) {
		sl := New[int, string]()

		// สร้าง mock random source
		// ลำดับแรกจะสร้าง level 5 (0,0,0,0 ทำให้ level เพิ่ม, 1 ทำให้หยุด)
		mockSource := &mockRandSource{
			// 256 (0b0100000000) จะทำให้ randomLevel คืนค่า 5
			// 1 (0b01) จะทำให้ randomLevel คืนค่า 1
			nums: []uint64{256, 1},
		}
		// แทนที่ random generator ของ skiplist ด้วย mock ของเรา
		sl.rand = rand.New(mockSource)

		// 1. Insert โหนดที่จะได้ level สูง (level 5)
		sl.Insert(100, "high-level-node")
		if sl.level != 5-1 { // sl.level เป็น 0-based index
			t.Fatalf("Expected skiplist level to be 4, but got %d", sl.level)
		}

		// 2. Delete โหนด เพื่อคืนโหนดที่มี forward slice ขนาดใหญ่กลับเข้า pool
		sl.Delete(100)

		// 3. Insert โหนดใหม่ ซึ่งจะได้ level ต่ำ (level 1)
		// การทำเช่นนี้จะดึงโหนดเก่าจาก pool มาใช้ และทำงานใน path `else` ที่เราต้องการทดสอบ
		sl.Insert(200, "reused-node")

		// 4. ตรวจสอบว่าการ insert สำเร็จ
		node, ok := sl.Search(200)
		if !ok || node.Value() != "reused-node" {
			t.Errorf("Expected to find key 200 with value 'reused-node', but got ok=%v, val='%s'", ok, node.Value())
		}
	})
}

func TestSkipList_PopMin(t *testing.T) {
	for _, setup := range getTestSetups[int, string]() {
		t.Run(setup.name, func(t *testing.T) {
			sl := setup.constructor(nil)

			// Test on empty list
			_, ok := sl.PopMin()
			if ok {
				t.Error("PopMin on empty list should return false")
			}

			sl.Insert(30, "thirty")
			sl.Insert(10, "ten")
			sl.Insert(20, "twenty")

			// Pop the smallest
			node, ok := sl.PopMin()
			if !ok || node.Key() != 10 || node.Value() != "ten" {
				t.Errorf("PopMin: Expected (10, 'ten'), got (%v, '%v', %v)", node.Key(), node.Value(), ok)
			}
			if sl.Len() != 2 {
				t.Errorf("Expected length 2 after PopMin, got %d", sl.Len())
			}
			_, found := sl.Search(10)
			if found {
				t.Error("Key 10 should not be found after PopMin")
			}

			// Pop the next smallest
			node, ok = sl.PopMin()
			if !ok || node.Key() != 20 || node.Value() != "twenty" {
				t.Errorf("PopMin: Expected (20, 'twenty'), got (%v, '%v', %v)", node.Key(), node.Value(), ok)
			}
			if sl.Len() != 1 {
				t.Errorf("Expected length 1 after PopMin, got %d", sl.Len())
			}

			// Pop the last one
			node, ok = sl.PopMin()
			if !ok || node.Key() != 30 || node.Value() != "thirty" {
				t.Errorf("PopMin: Expected (30, 'thirty'), got (%v, '%v', %v)", node.Key(), node.Value(), ok)
			}
			if sl.Len() != 0 {
				t.Errorf("Expected length 0 after PopMin, got %d", sl.Len())
			}

			// Test on empty list again
			_, ok = sl.PopMin()
			if ok {
				t.Error("PopMin on empty list after all elements popped should return false")
			}
		})
	}
}

func TestSkipList_PopMax(t *testing.T) {
	for _, setup := range getTestSetups[int, string]() {
		t.Run(setup.name, func(t *testing.T) {
			sl := setup.constructor(nil)

			// Test on empty list
			_, ok := sl.PopMax()
			if ok {
				t.Error("PopMax on empty list should return false")
			}

			sl.Insert(30, "thirty")
			sl.Insert(10, "ten")
			sl.Insert(20, "twenty")

			// Pop the largest
			node, ok := sl.PopMax()
			if !ok || node.Key() != 30 || node.Value() != "thirty" {
				t.Errorf("PopMax: Expected (30, 'thirty'), got (%v, '%v', %v)", node.Key(), node.Value(), ok)
			}
			if sl.Len() != 2 {
				t.Errorf("Expected length 2 after PopMax, got %d", sl.Len())
			}
			_, found := sl.Search(30)
			if found {
				t.Error("Key 30 should not be found after PopMax")
			}

			// Pop the next largest
			node, ok = sl.PopMax()
			if !ok || node.Key() != 20 || node.Value() != "twenty" {
				t.Errorf("PopMax: Expected (20, 'twenty'), got (%v, '%v', %v)", node.Key(), node.Value(), ok)
			}
			if sl.Len() != 1 {
				t.Errorf("Expected length 1 after PopMax, got %d", sl.Len())
			}

			// Pop the last one
			node, ok = sl.PopMax()
			if !ok || node.Key() != 10 || node.Value() != "ten" {
				t.Errorf("PopMax: Expected (10, 'ten'), got (%v, '%v', %v)", node.Key(), node.Value(), ok)
			}
			if sl.Len() != 0 {
				t.Errorf("Expected length 0 after PopMax, got %d", sl.Len())
			}

			// Test on empty list again
			_, ok = sl.PopMax()
			if ok {
				t.Error("PopMax on empty list after all elements popped should return false")
			}
		})
	}
}

func TestSkipList_CountRange(t *testing.T) {
	for _, setup := range getTestSetups[int, string]() {
		t.Run(setup.name, func(t *testing.T) {
			sl := setup.constructor(nil)
			sl.Insert(10, "ten")
			sl.Insert(20, "twenty")
			sl.Insert(30, "thirty")
			sl.Insert(40, "forty")
			sl.Insert(50, "fifty")

			tests := []struct {
				name  string
				start int
				end   int
				want  int
			}{
				{"Full range", 10, 50, 5},
				{"Partial range (middle)", 20, 40, 3},
				{"Partial range (start)", 5, 20, 2},
				{"Partial range (end)", 40, 55, 2},
				{"Single element range (exists)", 30, 30, 1},
				{"Single element range (not exists)", 25, 25, 0},
				{"Empty range (no elements)", 21, 29, 0},
				{"Range outside (before)", 1, 5, 0},
				{"Range outside (after)", 55, 60, 0},
				{"Invalid range (start > end)", 40, 20, 0},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					got := sl.CountRange(tt.start, tt.end)
					if got != tt.want {
						t.Errorf("CountRange(%d, %d): got %d, want %d", tt.start, tt.end, got, tt.want)
					}
				})
			}

			// Test with empty list
			emptySl := setup.constructor(nil)
			if emptySl.CountRange(10, 20) != 0 {
				t.Error("CountRange on empty list should return 0")
			}

			// Test after deletions
			sl.Delete(20)
			sl.Delete(40)
			if sl.CountRange(10, 50) != 3 { // 10, 30, 50
				t.Errorf("CountRange after deletions: got %d, want %d", sl.CountRange(10, 50), 3)
			}
			if sl.CountRange(15, 35) != 1 { // 30
				t.Errorf("CountRange after deletions: got %d, want %d", sl.CountRange(15, 35), 1)
			}
		})
	}
}

func TestSkipList_Iterator(t *testing.T) {
	for _, setup := range getTestSetups[int, string]() {
		t.Run(setup.name, func(t *testing.T) {
			sl := setup.constructor(nil)
			sl.Insert(10, "ten")
			sl.Insert(30, "thirty")
			sl.Insert(20, "twenty")
			sl.Insert(50, "fifty")
			sl.Insert(40, "forty")

			t.Run("Iterate from start", func(t *testing.T) {
				it := sl.NewIterator()
				var collectedKeys []int
				expectedKeys := []int{10, 20, 30, 40, 50}

				for it.Next() {
					collectedKeys = append(collectedKeys, it.Key())
				}

				if len(collectedKeys) != len(expectedKeys) {
					t.Fatalf("Expected %d keys, got %d. Keys: %v", len(expectedKeys), len(collectedKeys), collectedKeys)
				}
				for i, k := range collectedKeys {
					if k != expectedKeys[i] {
						t.Errorf("Expected key %d at index %d, got %d", expectedKeys[i], i, k)
					}
				}
			})

			t.Run("Reset", func(t *testing.T) {
				it := sl.NewIterator()
				it.Next() // Move to 10
				it.Next() // Move to 20

				it.Reset()
				// After reset, the first call to Next() should yield the first element
				if !it.Next() || it.Key() != 10 {
					t.Errorf("Reset failed: Expected to get key 10 after reset and Next(), but got something else")
				}
			})

			t.Run("Seek to existing key", func(t *testing.T) {
				it := sl.NewIterator()
				it.Seek(30)
				// First Next() after seek should land on the target key
				if !it.Next() || it.Key() != 30 || it.Value() != "thirty" {
					t.Errorf("Seek(30) failed: Expected to get (30, 'thirty'), but got (%v, '%v')", it.Key(), it.Value())
				}
				// Second Next() should move to the next element
				if !it.Next() || it.Key() != 40 {
					t.Errorf("After Seek(30) and Next(), expected to get key 40, but got %v", it.Key())
				}
			})

			t.Run("Seek to non-existing key (between)", func(t *testing.T) {
				it := sl.NewIterator()
				it.Seek(25) // Should seek to 30
				if !it.Next() || it.Key() != 30 {
					t.Errorf("Seek(25) failed: Expected to get key 30, but got %v", it.Key())
				}
			})

			t.Run("Seek to key smaller than min", func(t *testing.T) {
				it := sl.NewIterator()
				it.Seek(5) // Should seek to 10
				if !it.Next() || it.Key() != 10 {
					t.Errorf("Seek(5) failed: Expected to get key 10, but got %v", it.Key())
				}
			})

			t.Run("Seek to key larger than max", func(t *testing.T) {
				it := sl.NewIterator()
				it.Seek(55) // Should be invalid
				if it.Next() {
					t.Errorf("Seek(55) failed: Expected Next() to return false, but it returned true with key %v", it.Key())
				}
			})

			t.Run("First", func(t *testing.T) {
				it := sl.NewIterator()
				if !it.First() {
					t.Fatal("First() returned false on a non-empty list")
				}
				if it.Key() != 10 {
					t.Errorf("First(): Expected key 10, got %v", it.Key())
				}
				// Calling Next() after First() should move to the second element
				if !it.Next() || it.Key() != 20 {
					t.Errorf("Next() after First(): Expected key 20, got %v", it.Key())
				}

				// Test on empty list
				emptyIt := setup.constructor(nil).NewIterator()
				if emptyIt.First() {
					t.Error("First() on empty list should return false")
				}
			})

			t.Run("Last", func(t *testing.T) {
				it := sl.NewIterator()
				if !it.Last() {
					t.Fatal("Last() returned false on a non-empty list")
				}
				if it.Key() != 50 {
					t.Errorf("Last(): Expected key 50, got %v", it.Key())
				}
				// Calling Next() after Last() should return false
				if it.Next() {
					t.Errorf("Next() after Last() should return false, but it returned true with key %v", it.Key())
				}

				// Test on empty list
				emptyIt := setup.constructor(nil).NewIterator()
				if emptyIt.Last() {
					t.Error("Last() on empty list should return false")
				}
			})

			t.Run("Reverse iteration", func(t *testing.T) {
				it := sl.NewIterator()
				var collectedKeys []int
				expectedKeys := []int{50, 40, 30, 20, 10}

				// Start from the last element
				if !it.Last() {
					t.Fatal("Last() returned false on a non-empty list")
				}

				for {
					collectedKeys = append(collectedKeys, it.Key())
					if !it.Prev() {
						break
					}
				}

				if len(collectedKeys) != len(expectedKeys) {
					t.Fatalf("Reverse: Expected %d keys, got %d. Keys: %v", len(expectedKeys), len(collectedKeys), collectedKeys)
				}
				for i, k := range collectedKeys {
					if k != expectedKeys[i] {
						t.Errorf("Reverse: Expected key %d at index %d, got %d", expectedKeys[i], i, k)
					}
				}
			})

			t.Run("Clone", func(t *testing.T) {
				it1 := sl.NewIterator()
				it1.Next() // it1 is at 10
				it1.Next() // it1 is at 20

				it2 := it1.Clone() // it2 should also be at 20

				// Check that it2 is at the correct position
				if it2.Key() != 20 {
					t.Errorf("Cloned iterator should be at key 20, but got %v", it2.Key())
				}

				// Advance it1, it2 should not be affected
				it1.Next() // it1 is at 30
				if it1.Key() != 30 {
					t.Errorf("Original iterator failed to advance to 30, got %v", it1.Key())
				}
				if it2.Key() != 20 {
					t.Errorf("Cloned iterator was affected by original's Next(), expected 20, got %v", it2.Key())
				}

				// Advance it2, it1 should not be affected
				it2.Next() // it2 is at 30
				if it2.Key() != 30 {
					t.Errorf("Cloned iterator failed to advance to 30, got %v", it2.Key())
				}
				if it1.Key() != 30 {
					t.Errorf("Original iterator was affected by cloned's Next(), expected 30, got %v", it1.Key())
				}
			})
		})
	}
}

func TestSkipList_RangeWithIterator(t *testing.T) {
	for _, setup := range getTestSetups[int, string]() {
		t.Run(setup.name, func(t *testing.T) {
			sl := setup.constructor(nil)
			sl.Insert(10, "ten")
			sl.Insert(30, "thirty")
			sl.Insert(20, "twenty")
			sl.Insert(50, "fifty")
			sl.Insert(40, "forty")

			t.Run("Iterate all elements", func(t *testing.T) {
				var collectedKeys []int
				expectedKeys := []int{10, 20, 30, 40, 50}

				sl.RangeWithIterator(func(it *Iterator[int, string]) {
					for it.Next() {
						collectedKeys = append(collectedKeys, it.Key())
					}
				})

				if len(collectedKeys) != len(expectedKeys) {
					t.Fatalf("Expected %d keys, got %d. Keys: %v", len(expectedKeys), len(collectedKeys), collectedKeys)
				}
				for i, k := range collectedKeys {
					if k != expectedKeys[i] {
						t.Errorf("Expected key %d at index %d, got %d", expectedKeys[i], i, k)
					}
				}
			})

			t.Run("Seek and iterate", func(t *testing.T) {
				var collectedKeys []int
				expectedKeys := []int{30, 40, 50}

				sl.RangeWithIterator(func(it *Iterator[int, string]) {
					it.Seek(25) // Should position before 30
					for it.Next() {
						collectedKeys = append(collectedKeys, it.Key())
					}
				})

				if len(collectedKeys) != len(expectedKeys) {
					t.Fatalf("Seek: Expected %d keys, got %d. Keys: %v", len(expectedKeys), len(collectedKeys), collectedKeys)
				}
				for i, k := range collectedKeys {
					if k != expectedKeys[i] {
						t.Errorf("Seek: Expected key %d at index %d, got %d", expectedKeys[i], i, k)
					}
				}
			})

			t.Run("Empty list", func(t *testing.T) {
				emptySl := setup.constructor(nil)
				wasCalled := false
				// The callback should be called, but the loop inside should not execute.
				emptySl.RangeWithIterator(func(it *Iterator[int, string]) {
					wasCalled = true
					if it.Next() {
						t.Error("Next() on empty list should return false")
					}
				})
				if !wasCalled {
					t.Error("Callback for RangeWithIterator was not called on an empty list")
				}
			})
		})
	}
}
