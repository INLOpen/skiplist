# Go Generic Skiplist

[![Go](https://github.com/INLOpen/skiplist/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/INLOpen/skiplist/actions/workflows/go.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/INLOpen/skiplist)](https://goreportcard.com/report/github.com/INLOpen/skiplist) [![Go Reference](https://pkg.go.dev/badge/github.com/INLOpen/skiplist.svg)](https://pkg.go.dev/github.com/INLOpen/skiplist)

A thread-safe, high-performance, generic skiplist implementation in Go.

This library provides a skiplist data structure that is easy to use, highly efficient, and flexible. It's designed to be a powerful alternative to Go's built-in `map` when you need sorted, ordered data access with logarithmic time complexity for search, insert, and delete operations.

A skiplist maintains sorted order by using a hierarchy of linked lists. Lower levels have more elements, creating a dense index, while higher levels have fewer elements, acting as "express lanes" to skip over large portions of the list. This structure enables fast searching.

```
Level 3: 1-------------------------------->9
          |                                |
Level 2: 1--------->4--------------------->9
          |          |                     |
Level 1: 1--->2----->4--------->6--------->9
          |   |      |          |         |
Data:    (1) (2)    (4)        (6)       (9)
```

## Features

*   **🚀 High Performance**: O(log n) average time complexity for major operations.
*   **🧠 Memory Optimized**:
	*   **`sync.Pool` (Default)**: Recycles nodes to reduce memory allocations and GC pressure, ideal for high-churn workloads.
	*   **⚡️ Optional Memory Arena**: For maximum performance and minimal GC impact, an optional lock-free memory arena allocator can be enabled.
*   **⛓️ Generic**: Fully generic (`[K any, V any]`), allowing any type for keys and values.
*   **🎛️ Customizable Sorting**: Supports custom comparator functions, enabling complex sorting logic for any key type (e.g., `structs`).
*   **🤝 Thread-Safe**: All operations are safe for concurrent use from multiple goroutines.
*   **✨ Rich API**: Includes a comprehensive set of methods like `RangeQuery`, `PopMin`, `PopMax`, `Predecessor`, `Successor`, and more.
*   **🚶 Full-Featured Iterator**: Provides a powerful bidirectional iterator with `Seek`, `Next`, `Prev`, `First`, `Last`, `Reset`, etc., for flexible data traversal.

## Why Use This Skip List?

While Go's built-in `map` is excellent for general-purpose key-value storage, it does not maintain any order. This skiplist is superior in scenarios where sorted data is crucial.

| Use Case | `map[K]V` | `sync.Map` | `This SkipList[K, V]` |
|---|---|---|---|
| **Unordered Key-Value** | ✅ **Best Choice** | ✅ (Concurrent) | (Overhead) |
| **Ordered Iteration** | ❌ (Unordered) | ❌ (Unordered) | ✅ **Best Choice** |
| **Find Min/Max Key** | ❌ (Requires full scan) | ❌ (Requires full scan) | ✅ **O(1)** |
| **Range Queries (e.g., keys 10-50)** | ❌ (Requires full scan) | ❌ (Requires full scan) | ✅ **O(log n + k)** |
| **Find Predecessor/Successor** | ❌ (Unordered) | ❌ (Unordered) | ✅ **O(log n)** |
| **Fine-grained GC Control** | ❌ | ❌ | ✅ (via `sync.Pool` or Arena) |

## Performance

This skiplist offers two memory allocation strategies, each with distinct performance characteristics.

*   **`sync.Pool` (Default)**: This is the standard, memory-efficient choice. It excels in high-churn workloads (frequent inserts and deletes) by recycling nodes, which significantly reduces the garbage collector's workload.
*   **`Memory Arena` (Optional)**: This is the high-throughput choice. By pre-allocating a large, contiguous block of memory, it nearly eliminates GC overhead for node allocations, resulting in lower and more predictable latency for bulk operations. It uses more memory upfront and is less ideal for high-churn workloads where nodes are not reclaimed individually.

**Conclusion**:
*   Use the **default `sync.Pool`** for general-purpose use and high-churn scenarios (frequent inserts/deletes).
*   Use the **`Memory Arena`** for the absolute lowest latency during bulk inserts/reads, especially when you can predict memory usage.

*Results on `13th Gen Intel(R) Core(TM) i9-13900H`*

| Benchmark (ns/op) | `sync.Pool` (Default) | `Memory Arena` | Notes |
|---|---|---|---|
| **Bulk Insert (100k items)** | 106,216,625 | 120,044,970 | Total time to insert 100,000 items |
| **Search (avg)** | 286.1 | 351.8 | Average time per search in a list of 100k items |
| **Delete (avg)** | 2,446 | 2,227 | Average time per delete in a list of 100k items |
| **Ordered Iteration (10k items)** | 213,295 | 181,974 | Total time to iterate over 10,000 items |
| **Churn (delete/insert)** | 271.3 | N/A | Arena is not optimized for high churn; no node reclamation |
| **Warm Insert (1 op)** | 84.50 | N/A | `sync.Pool` excels at reusing nodes for single inserts |

## Installation

```bash
$ go get github.com/INLOpen/skiplist
```

## Usage

### Basic Usage (Ordered Keys)

For standard key types that support ordering (like `int`, `string`), you can use the simple `New()` constructor.

```go
package main

import (
	"fmt"
	"github.com/INLOpen/skiplist"
)

func main() {
	// Create a new skiplist for int keys and string values.
	// The default comparator (cmp.Compare) is used automatically.
	sl := skiplist.New[int, string]()

	sl.Insert(10, "ten")
	sl.Insert(20, "twenty")
	sl.Insert(30, "thirty")

	// Search for a value
	node, ok := sl.Search(20)
	if ok {
		fmt.Printf("Found key 20 with value: %s\n", node.Value()) // "twenty"
	}

	// Iterate over all items in sorted order
	fmt.Println("All items:")
	sl.Range(func(key int, value string) bool {
		fmt.Printf("  %d: %s\n", key, value)
		return true // Continue iteration
	})

	// Pop the maximum element
	maxNode, ok := sl.PopMax()
	if ok {
		fmt.Printf("Popped max element: %d -> %s\n", maxNode.Key(), maxNode.Value()) // 30 -> "thirty"
	}

	fmt.Printf("Current length: %d\n", sl.Len()) // 2
}
```

### High-Performance Usage (with Memory Arena)

For scenarios demanding the lowest possible latency, such as bulk loading data, you can enable the memory arena.

```go
package main

import (
	"fmt"
	"github.com/INLOpen/skiplist"
)

func main() {
	// For maximum performance, create a skiplist with a 128MB memory arena.
	// This is ideal for scenarios where you insert a large number of items
	// and want to minimize garbage collection overhead.
	arenaOpt := skiplist.WithArena[int, string](128 * 1024 * 1024) // 128MB
	sl := skiplist.New[int, string](arenaOpt)

	// Operations are the same
	sl.Insert(1, "one")
	fmt.Println("Length with Arena:", sl.Len())
}
```

### Custom Comparator (Struct Keys)

To use a custom struct as a key, you must provide your own comparator function to `NewWithComparator`.

```go
package main

import (
	"fmt"
	"github.com/INLOpen/skiplist"
)

type User struct {
	ID   int
	Name string
}

// userComparator defines how to sort User keys (by ID).
func userComparator(a, b User) int {
	if a.ID < b.ID {
		return -1
	}
	if a.ID > b.ID {
		return 1
	}
	return 0
}

func main() {
	// Create a skiplist with a custom comparator
	sl := skiplist.NewWithComparator[User, string](userComparator)

	sl.Insert(User{ID: 2, Name: "Bob"}, "Engineer")
	sl.Insert(User{ID: 1, Name: "Alice"}, "Manager")

	// The list is sorted by User.ID. Min() returns an INode.
	minNode, ok := sl.Min()
	if ok {
		userKey := minNode.Key()
		fmt.Printf("Min user is: %s (ID: %d), Role: %s\n", userKey.Name, userKey.ID, minNode.Value()) // "Alice (ID: 1), Role: Manager"
	}
}
```

### Concurrent Usage

The skiplist is safe for concurrent use. You can have multiple goroutines reading and writing to the list simultaneously without external locking.

```go
package main

import (
	"fmt"
	"github.com/INLOpen/skiplist"
	"sync"
)

func main() {
	sl := skiplist.New[int, int]()
	var wg sync.WaitGroup

	// Concurrently insert 1000 items
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(val int) {
			defer wg.Done()
			sl.Insert(val, val*10)
		}(i)
	}

	wg.Wait()
	fmt.Printf("After concurrent inserts, length is: %d\n", sl.Len()) // 1000
}
```

### Iterator Usage

The iterator provides fine-grained control over list traversal.

```go
package main

import (
	"fmt"
	"github.com/INLOpen/skiplist"
)

func main() {
	sl := skiplist.New[int, string]()
	sl.Insert(10, "A")
	sl.Insert(20, "B")
	sl.Insert(30, "C")
	sl.Insert(40, "D")

	// Create a new iterator
	it := sl.NewIterator()

	// Seek to the first element >= 25
	it.Seek(25)

	// Iterate from the seeked position
	fmt.Println("Iterating from key 25 onwards:")
	for it.Next() {
		fmt.Printf("  %d: %s\n", it.Key(), it.Value())
	}
	// Output:
	//   30: C
	//   40: D
}
```

## API Reference

### Constructors
*   `New[K cmp.Ordered, V any](opts ...Option[K, V]) *SkipList[K, V]`
*   `NewWithComparator[K any, V any](compare Comparator[K], opts ...Option[K, V]) *SkipList[K, V]`

### Configuration Options
*   `WithArenaK, V Option[K, V]`

### Basic Operations
*   `(sl *SkipList[K, V]) Insert(key K, value V) INode[K, V]`
*   `(sl *SkipList[K, V]) Search(key K) (INode[K, V], bool)`
*   `(sl *SkipList[K, V]) Delete(key K) bool`
*   `(sl *SkipList[K, V]) Len() int`
*   `(sl *SkipList[K, V]) Clear()`

### Ordered Operations
*   `(sl *SkipList[K, V]) Min() (INode[K, V], bool)`
*   `(sl *SkipList[K, V]) Max() (INode[K, V], bool)`
*   `(sl *SkipList[K, V]) PopMin() (INode[K, V], bool)`
*   `(sl *SkipList[K, V]) PopMax() (INode[K, V], bool)`
*   `(sl *SkipList[K, V]) Predecessor(key K) (INode[K, V], bool)`
*   `(sl *SkipList[K, V]) Successor(key K) (INode[K, V], bool)`
*   `(sl *SkipList[K, V]) Seek(key K) (INode[K, V], bool)`

### Iteration & Range
*   `(sl *SkipList[K, V]) Range(f func(key K, value V) bool)`
*   `(sl *SkipList[K, V]) RangeQuery(start, end K, f func(key K, value V) bool)`
*   `(sl *SkipList[K, V]) CountRange(start, end K) int`
*   `(sl *SkipList[K, V]) NewIterator() *Iterator[K, V]`
*   `(sl *SkipList[K, V]) RangeWithIterator(f func(it *Iterator[K, V]))`

### Iterator Methods
*   `(it *Iterator[K, V]) Next() bool`
*   `(it *Iterator[K, V]) Prev() bool`
*   `(it *Iterator[K, V]) Key() K`
*   `(it *Iterator[K, V]) Value() V`
*   `(it *Iterator[K, V]) Seek(key K) bool`
*   `(it *Iterator[K, V]) SeekToFirst() bool`
*   `(it *Iterator[K, V]) SeekToLast() bool`
*   `(it *Iterator[K, V]) First() bool`
*   `(it *Iterator[K, V]) Last() bool`
*   `(it *Iterator[K, V]) Reset()`
*   `(it *Iterator[K, V]) Clone() *Iterator[K, V]`

## Contributing

Contributions are welcome! Please feel free to submit a pull request or open an issue.

## License

This project is licensed under the MIT License - see the LICENSE.md file for details.
