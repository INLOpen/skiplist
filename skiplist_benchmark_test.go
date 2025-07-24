package skiplist

import (
	"math/rand/v2"
	"testing"
)

const insertBenchmarkSize = 100000
const benchmarkSize = 10000 // Increase size for more realistic benchmarks

// generateRandomKeys generates a slice of unique random integers.
func generateRandomKeys(n int) []int {
	keys := make([]int, n)
	seen := make(map[int]struct{})
	r := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())) // Use a new random source for reproducibility in benchmarks if needed, or just for good practice.

	for i := 0; i < n; {
		key := r.IntN(n * 10) // Generate keys in a wider range to avoid too many collisions if N is small
		if _, ok := seen[key]; !ok {
			keys[i] = key
			seen[key] = struct{}{}
			i++
		}
	}
	return keys
}

// BenchmarkSkipList_Insert measures the average performance of inserting a single element
// into a skiplist that is growing from 0 to N elements.
func BenchmarkSkipList_Insert(b *testing.B) {
	b.StopTimer()
	keys := generateRandomKeys(b.N)
	sl := New[int, int]()
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		sl.Insert(keys[i], i)
	}
}

// BenchmarkMap_Insert measures the average performance of inserting a single element
// into a map that is growing from 0 to N elements.
func BenchmarkMap_Insert(b *testing.B) {
	b.StopTimer()
	keys := generateRandomKeys(b.N)
	m := make(map[int]int, b.N) // Pre-allocate map capacity
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		m[keys[i]] = i
	}
}

func BenchmarkSkipList_Search(b *testing.B) {
	for _, setup := range getTestSetups[int, int]() {
		b.Run(setup.name, func(b *testing.B) {
			keys := generateRandomKeys(benchmarkSize)
			sl := setup.constructor(nil)
			b.StopTimer()
			for j := 0; j < benchmarkSize; j++ {
				sl.Insert(keys[j], keys[j])
			}

			b.StartTimer()
			for i := 0; i < b.N; i++ {
				_, _ = sl.Search(keys[i%benchmarkSize])
			}
		})
	}
}

// BenchmarkInsertN_WithPool วัดประสิทธิภาพการเพิ่มข้อมูล N รายการโดยใช้ sync.Pool
func BenchmarkInsertN_WithPool(b *testing.B) {
	keys := generateRandomKeys(insertBenchmarkSize)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// ในแต่ละรอบของ benchmark, สร้าง list ใหม่และเติมข้อมูลเข้าไป
		sl := New[int, int]() // ใช้ค่าเริ่มต้น (sync.Pool)
		for j := 0; j < insertBenchmarkSize; j++ {
			sl.Insert(keys[j], keys[j])
		}
	}
}

// BenchmarkInsertN_WithArena วัดประสิทธิภาพการเพิ่มข้อมูล N รายการโดยใช้ Memory Arena
func BenchmarkInsertN_WithArena(b *testing.B) {
	keys := generateRandomKeys(insertBenchmarkSize)
	// ประเมินขนาดของ Arena ที่ต้องใช้คร่าวๆ
	// ขนาด node struct + ขนาด slice header + ขนาด slice data (MaxLevel * 8 bytes)
	// สมมติว่าประมาณ 400 bytes ต่อโหนดเพื่อความปลอดภัย
	arenaSizeBytes := insertBenchmarkSize * 400
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// ในแต่ละรอบของ benchmark, สร้าง list ใหม่พร้อม Arena และเติมข้อมูลเข้าไป
		sl := New(WithArena[int, int](arenaSizeBytes)) // ใช้ Functional Option
		for j := 0; j < insertBenchmarkSize; j++ {
			sl.Insert(keys[j], keys[j])
		}
	}
}

func BenchmarkMap_Search(b *testing.B) {
	keys := generateRandomKeys(benchmarkSize)
	m := make(map[int]int)
	b.StopTimer()
	for j := 0; j < benchmarkSize; j++ {
		m[keys[j]] = keys[j]
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_ = m[keys[i%benchmarkSize]]
	}
}

// BenchmarkMap_Churn measures the performance of a map under high churn conditions
// (frequent deletions and insertions) to provide a comparison for SkipList's churn performance.
func BenchmarkMap_Churn(b *testing.B) {
	keys := generateRandomKeys(benchmarkSize)
	m := make(map[int]int)
	b.StopTimer()
	for _, key := range keys {
		m[key] = key
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		keyToDelete := keys[i%benchmarkSize]
		delete(m, keyToDelete)
		// Insert a new key to avoid hitting the same memory location and better simulate churn.
		keyToInsert := keyToDelete + (benchmarkSize * 10)
		m[keyToInsert] = keyToInsert
	}
}

// BenchmarkSkipList_Insert_SingleOp_Warm measures the cost of a single insert operation
// when the node pool is expected to be warm (nodes are reused).
// It measures the cost of an insert-delete cycle.
func BenchmarkSkipList_Insert_SingleOp_Warm(b *testing.B) {
	// This benchmark is specific to the pool allocator's behavior.
	b.Run("WithPool", func(b *testing.B) {
		sl := New[int, int]()
		b.StopTimer()
		warmupKeys := generateRandomKeys(benchmarkSize)
		for _, key := range warmupKeys {
			sl.Insert(key, key)
		}
		for _, key := range warmupKeys {
			sl.Delete(key)
		}

		b.StartTimer()
		for i := 0; i < b.N; i++ {
			sl.Insert(i, i)
			sl.Delete(i)
		}
	})
}

// BenchmarkSkipList_Churn tests the performance under high churn conditions
// (frequent insertions and deletions), which highlights the benefits of sync.Pool.
func BenchmarkSkipList_Churn(b *testing.B) {
	// This benchmark is specifically designed to test workloads with high churn,
	// which is a primary use case for sync.Pool. An arena allocator is not
	// designed for this pattern, as it doesn't reclaim individual nodes on Put().
	// Therefore, we only run this for the pool-based allocator.
	b.Run("WithPool", func(b *testing.B) {
		keys := generateRandomKeys(benchmarkSize)
		sl := New[int, int]()
		b.StopTimer()
		for _, key := range keys {
			sl.Insert(key, key)
		}

		b.StartTimer()
		for i := 0; i < b.N; i++ {
			keyToDelete := keys[i%benchmarkSize]
			sl.Delete(keyToDelete)
			keyToInsert := keys[(i+1)%benchmarkSize] + benchmarkSize*10
			sl.Insert(keyToInsert, keyToInsert)
		}
	})
}

// BenchmarkSkipList_Range measures the performance of iterating through all elements
// in the skiplist using the Range function.
func BenchmarkSkipList_Range(b *testing.B) {
	for _, setup := range getTestSetups[int, int]() {
		b.Run(setup.name, func(b *testing.B) {
			sl := setup.constructor(nil)
			keys := generateRandomKeys(benchmarkSize)
			b.StopTimer()
			for _, key := range keys {
				sl.Insert(key, key)
			}

			b.StartTimer()
			for i := 0; i < b.N; i++ {
				sl.Range(func(key int, value int) bool { return true })
			}
		})
	}
}

// BenchmarkSkipList_Iterator_Safe measures the performance of iterating through all elements
// using the standard, thread-safe iterator, which acquires a lock on each operation.
func BenchmarkSkipList_Iterator_Safe(b *testing.B) {
	for _, setup := range getTestSetups[int, int]() {
		b.Run(setup.name, func(b *testing.B) {
			sl := setup.constructor(nil)
			keys := generateRandomKeys(benchmarkSize)
			b.StopTimer()
			for _, key := range keys {
				sl.Insert(key, key)
			}
			b.StartTimer()

			for i := 0; i < b.N; i++ {
				it := sl.NewIterator()
				for it.Next() {
					_ = it.Key()
					_ = it.Value()
				}
			}
		})
	}
}

// BenchmarkSkipList_RangeWithIterator measures the performance of iterating through all elements
// using the more efficient RangeWithIterator method, which holds a single read lock
// and uses an internal "unsafe" iterator.
func BenchmarkSkipList_RangeWithIterator(b *testing.B) {
	for _, setup := range getTestSetups[int, int]() {
		b.Run(setup.name, func(b *testing.B) {
			sl := setup.constructor(nil)
			keys := generateRandomKeys(benchmarkSize)
			b.StopTimer()
			for _, key := range keys {
				sl.Insert(key, key)
			}
			b.StartTimer()

			for i := 0; i < b.N; i++ {
				sl.RangeWithIterator(func(it *Iterator[int, int]) {
					for it.Next() {
						_ = it.Key()
						_ = it.Value()
					}
				})
			}
		})
	}
}

// BenchmarkSkipList_Clear measures the performance of clearing a pre-filled skiplist.
// This benchmark focuses on the time taken to reset the list's state and replace the node pool.
// The actual garbage collection of old nodes happens asynchronously and is not measured here.
//
// NOTE: The original implementation of this benchmark caused timeouts. The Clear() operation
// is extremely fast. The benchmark runner would try to execute it millions of times (by increasing b.N)
// to get a stable reading. However, the setup for each operation (filling the list) was
// untimed but still inside the b.N loop. This caused the slow setup code to run for a
// very long time, leading to a timeout.
//
// The corrected benchmark below measures the entire cycle of creating, filling, and then
// clearing a skiplist. This avoids the timeout issue. The cost of Clear() itself is
// negligible compared to the cost of filling the list.
func BenchmarkSkipList_Clear(b *testing.B) {
	for _, setup := range getTestSetups[int, int]() {
		// This benchmark now measures the cost of clearing a list and refilling it,
		// which is a more stable and realistic workload than creating a new list every time.
		b.Run(setup.name, func(b *testing.B) {
			sl := setup.constructor(nil)
			keys := generateRandomKeys(benchmarkSize)
			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				// The timed operation is clearing the list and then refilling it.
				sl.Clear()
				for _, key := range keys {
					sl.Insert(key, key)
				}
			}
		})
	}
}
