# Go Generic Skiplist

[![Go](https://github.com/INLOpen/skiplist/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/INLOpen/skiplist/actions/workflows/go.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/INLOpen/skiplist)](https://goreportcard.com/report/github.com/INLOpen/skiplist) [![Go Reference](https://pkg.go.dev/badge/github.com/INLOpen/skiplist.svg)](https://pkg.go.dev/github.com/INLOpen/skiplist)

A thread-safe, high-performance, generic skiplist implementation in Go.

This library provides a skiplist data structure that is easy to use, highly efficient, and flexible. It's designed to be a powerful alternative to Go's built-in `map` when you need sorted, ordered data access with logarithmic time complexity for search, insert, and delete operations.

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

## Performance

This skiplist offers two memory allocation strategies, each with distinct performance characteristics.

*   **`sync.Pool` (Default)**: This is the standard, memory-efficient choice. It excels in high-churn workloads (frequent inserts and deletes) by recycling nodes, which significantly reduces the garbage collector's workload.
*   **`Memory Arena` (Optional)**: This is the high-throughput choice. By pre-allocating a large, contiguous block of memory, it nearly eliminates GC overhead for node allocations, resulting in lower and more predictable latency for bulk operations. It uses more memory upfront and is less ideal for high-churn workloads where nodes are not reclaimed individually.

**Conclusion**:
*   Use the **default `sync.Pool`** for general-purpose use and high-churn scenarios.
*   Use the **`Memory Arena`** when you need the absolute lowest latency for bulk inserts/deletes and can predict memory usage.
*   Choose this skiplist over a `map` when you need **sorted data**, **ordered iteration (Range queries)**, or **fine-grained control over memory allocation and GC pressure**.

*Results on `13th Gen Intel(R) Core(TM) i9-13900H`*

| Benchmark (ns/op) | `sync.Pool` (Default) | `Memory Arena` | Notes |
|---|---|---|---|
| **Bulk Insert (100k items)** | 106,216,625 | 120,044,970 | Time to insert 100k items |
| **Search (avg)** | 286.1 | 351.8 | Average time per search |
| **Delete (avg)** | 2,446 | 2,227 | Average time per delete |
| **Ordered Iteration (10k items)** | 213,295 | 181,974 | Time for full scan with `RangeWithIterator` |
| **Churn (delete/insert)** | 271.3 | N/A | Arena not designed for this workload |
| **Warm Insert (1 op)** | 84.50 | N/A | Highlights `sync.Pool`'s reuse speed |

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
*   `WithArenaK any, V any Option[K, V]`

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
*   `(it *Iterator[K, V]) Seek(key K)`
*   `(it *Iterator[K, V]) First() bool`
*   `(it *Iterator[K, V]) Last() bool`
*   `(it *Iterator[K, V]) SeekToFirst()`
*   `(it *Iterator[K, V]) SeekToLast()`
*   `(it *Iterator[K, V]) Reset()`
*   `(it *Iterator[K, V]) Clone() *Iterator[K, V]`

## Contributing

Contributions are welcome! Please feel free to submit a pull request or open an issue.

## License

This project is licensed under the MIT License - see the LICENSE.md file for details.
