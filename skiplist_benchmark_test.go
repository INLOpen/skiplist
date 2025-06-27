package skiplist

import (
	"math/rand/v2"
	"testing"
)

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

const benchmarkSize = 10000 // Number of items to insert/search/delete

func BenchmarkSkipList_Insert(b *testing.B) {
	keys := generateRandomKeys(benchmarkSize)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sl := New[int, int]() // สร้าง SkipList ใหม่ในแต่ละ iteration
		for j := 0; j < benchmarkSize; j++ {
			sl.Insert(keys[j], j)
		}
	}
}

func BenchmarkMap_Insert(b *testing.B) {
	keys := generateRandomKeys(benchmarkSize)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := make(map[int]int)
		for j := 0; j < benchmarkSize; j++ {
			m[keys[j]] = keys[j]
		}
	}
}

func BenchmarkSkipList_Search(b *testing.B) {
	keys := generateRandomKeys(benchmarkSize)
	sl := New[int, int]() // สร้าง SkipList ใหม่ในแต่ละ iteration
	b.StopTimer()         // Stop timer for setup
	for j := 0; j < benchmarkSize; j++ {
		sl.Insert(keys[j], keys[j])
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = sl.Search(keys[i%benchmarkSize]) // Ignore returned node
	} // b.StartTimer() is implicitly called here
}

func BenchmarkMap_Search(b *testing.B) {
	keys := generateRandomKeys(benchmarkSize)
	m := make(map[int]int)
	b.StopTimer() // Stop timer for setup
	for j := 0; j < benchmarkSize; j++ {
		m[keys[j]] = keys[j]
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m[keys[i%benchmarkSize]]
	} // b.StartTimer() is implicitly called here
}

func BenchmarkSkipList_Delete(b *testing.B) {
	keys := generateRandomKeys(benchmarkSize)
	sl := New[int, int]() // สร้าง SkipList ใหม่ในแต่ละ iteration
	b.StopTimer()         // Stop timer for setup
	// Fill the skiplist before starting the benchmark
	for j := 0; j < benchmarkSize; j++ {
		sl.Insert(keys[j], keys[j])
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sl.Delete(keys[i%benchmarkSize])
		sl.Insert(keys[i%benchmarkSize], keys[i%benchmarkSize]) // Re-insert to maintain size for next iteration
	} // b.StartTimer() is implicitly called here
}

func BenchmarkMap_Delete(b *testing.B) {
	keys := generateRandomKeys(benchmarkSize)
	m := make(map[int]int)
	b.StopTimer() // Stop timer for setup
	for j := 0; j < benchmarkSize; j++ {
		m[keys[j]] = keys[j]
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		delete(m, keys[i%benchmarkSize])
		m[keys[i%benchmarkSize]] = keys[i%benchmarkSize] // Re-insert to maintain size for next iteration
	} // b.StartTimer() is implicitly called here
}

// BenchmarkSkipList_Insert_SingleOp_Warm measures the cost of a single insert operation
// when the node pool is expected to be warm (nodes are reused).
// It measures the cost of an insert-delete cycle.
// [Minor: Added comment to clarify what this benchmark measures]
func BenchmarkSkipList_Insert_SingleOp_Warm(b *testing.B) {
	sl := New[int, int]() // สร้าง SkipList ใหม่ในแต่ละ iteration
	// Pre-fill and clear the list to warm up the pool
	warmupKeys := generateRandomKeys(benchmarkSize)
	for _, key := range warmupKeys {
		sl.Insert(key, key)
	}
	b.StopTimer() // Stop timer for setup
	for _, key := range warmupKeys {
		sl.Delete(key)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sl.Insert(i, i) // Insert a new key (this is the operation being measured)
		sl.Delete(i)    // Delete it immediately to return node to pool (this is part of the measured cycle)
	} // b.StartTimer() is implicitly called here
}

// BenchmarkSkipList_Churn tests the performance under high churn conditions
// (frequent insertions and deletions), which highlights the benefits of sync.Pool.
func BenchmarkSkipList_Churn(b *testing.B) {
	keys := generateRandomKeys(benchmarkSize)
	sl := New[int, int]() // สร้าง SkipList ใหม่ในแต่ละ iteration
	b.StopTimer()         // Stop timer for setup
	// Pre-fill the skiplist
	for _, key := range keys {
		sl.Insert(key, key)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// In each iteration, delete a key and insert a new one.
		// This simulates a workload where the data set is constantly changing.
		keyToDelete := keys[i%benchmarkSize]
		sl.Delete(keyToDelete)

		// Use a different key for insertion to avoid simply replacing the deleted one.
		// The range of new keys is outside the initial set.
		keyToInsert := keys[(i+1)%benchmarkSize] + benchmarkSize*10
		sl.Insert(keyToInsert, keyToInsert) // This is the operation being measured
	} // b.StartTimer() is implicitly called here
}

// BenchmarkSkipList_Range measures the performance of iterating through all elements
// in the skiplist using the Range function.
// [Minor: Added comment to clarify what this benchmark measures]
func BenchmarkSkipList_Range(b *testing.B) {
	sl := New[int, int]() // สร้าง SkipList ใหม่ในแต่ละ iteration
	// Pre-fill the skiplist with benchmarkSize items
	keys := generateRandomKeys(benchmarkSize)
	b.StopTimer() // Stop timer for setup
	for _, key := range keys {
		sl.Insert(key, key)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sl.Range(func(key int, value int) bool { return true }) // Iterate through all elements
	} // b.StartTimer() is implicitly called here
}
