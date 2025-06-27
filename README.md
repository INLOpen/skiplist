# Go Generic Skiplist

[![Go](https://github.com/INLOpen/skiplist/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/INLOpen/skiplist/actions/workflows/go.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/INLOpen/skiplist)](https://goreportcard.com/report/github.com/INLOpen/skiplist) [![Go Reference](https://pkg.go.dev/badge/github.com/INLOpen/skiplist.svg)](https://pkg.go.dev/github.com/INLOpen/skiplist)

A thread-safe, high-performance, generic skiplist implementation in Go.

This library provides a skiplist data structure that is easy to use, highly efficient, and flexible. It's designed to be a powerful alternative to Go's built-in `map` when you need sorted, ordered data access with logarithmic time complexity for search, insert, and delete operations.

## Features

*   **🚀 High Performance**: O(log n) average time complexity for major operations.
*   **🧠 Memory Optimized**: Utilizes `sync.Pool` to recycle nodes, significantly reducing memory allocations and GC pressure in high-churn scenarios.
*   **⛓️ Generic**: Fully generic (`[K any, V any]`), allowing any type for keys and values.
*   **🎛️ Customizable Sorting**: Supports custom comparator functions, enabling complex sorting logic for any key type (e.g., `structs`).
*   **🤝 Thread-Safe**: All operations are safe for concurrent use from multiple goroutines.
*   **✨ Rich API**: Includes a comprehensive set of methods like `RangeQuery`, `PopMin`, `PopMax`, `Predecessor`, `Successor`, and more.
*   **🚶 Full-Featured Iterator**: Provides a powerful iterator with `Seek`, `Next`, `Rewind`, etc., for flexible data traversal.

## Installation

```sh
go get github.com/INLOpen/skiplist
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

	sl.Insert(30, "thirty")
	sl.Insert(10, "ten")
	sl.Insert(20, "twenty")

	// Search for a value
	node, ok := sl.Search(20) // Search now returns *Node[K,V], bool
	if ok {
		fmt.Printf("Found key 20 with value: %s\n", node.Value) // "twenty"
	}

	// Iterate over all items in sorted order
	fmt.Println("All items:")
	sl.Range(func(key int, value string) bool {
		fmt.Printf("  %d: %s\n", key, value)
		return true // Continue iteration
	})

	// Pop the maximum element (PopMax now returns *Node[K,V], bool)
	maxNode, ok := sl.PopMax()
	if ok {
		fmt.Printf("Popped max element: %d -> %s\n", maxNode.Key, maxNode.Value) // 30 -> "thirty"
	}

	fmt.Printf("Current length: %d\n", sl.Len()) // 2
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

	// The list is sorted by User.ID
	minUser, _, _ := sl.Min()
	fmt.Printf("Min user is: %s (ID: %d)\n", minUser.Name, minUser.ID) // "Alice (ID: 1)"
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
	sl.Insert(40, "D") // [Minor: This line was missing in the original diff, adding it for completeness]

	// Create a new iterator
	it := sl.NewIterator()

	// Seek to the first element >= 25
	it.Seek(25)

	// Iterate from the seeked position
	fmt.Println("Iterating from key 25 onwards:")
	for it.Valid() {
		fmt.Printf("  %d: %s\n", it.Key(), it.Value())
		it.Next()
	}
	// Output:
	//   30: C
	//   40: D
}
```

## API Reference

### Constructors
*   `NewK cmp.Ordered, V any *SkipList[K, V]`
*   `NewWithComparator[K any, V any](compare Comparator[K]) *SkipList[K, V]`

### Basic Operations
*   `(sl *SkipList[K, V]) Insert(key K, value V)`
*   `(sl *SkipList[K, V]) Search(key K) (*Node[K, V], bool)`
*   `(sl *SkipList[K, V]) Delete(key K) bool`
*   `(sl *SkipList[K, V]) Len() int`

### Ordered Operations
*   `(sl *SkipList[K, V]) Min() (*Node[K, V], bool)`
*   `(sl *SkipList[K, V]) Max() (*Node[K, V], bool)`
*   `(sl *SkipList[K, V]) PopMin() (*Node[K, V], bool)`
*   `(sl *SkipList[K, V]) PopMax() (*Node[K, V], bool)`
*   `(sl *SkipList[K, V]) Predecessor(key K) (*Node[K, V], bool)`
*   `(sl *SkipList[K, V]) Successor(key K) (*Node[K, V], bool)`

### Iteration & Range
*   `(sl *SkipList[K, V]) Range(f func(key K, value V) bool)`
*   `(sl *SkipList[K, V]) RangeQuery(start, end K, f func(key K, value V) bool)`
*   `(sl *SkipList[K, V]) CountRange(start, end K) int`
*   `(sl *SkipList[K, V]) NewIterator() *Iterator[K, V]`

### Iterator Methods
*   `(it *Iterator[K, V]) Valid() bool`
*   `(it *Iterator[K, V]) Key() K`
*   `(it *Iterator[K, V]) Value() V`
*   `(it *Iterator[K, V]) Next()`
*   `(it *Iterator[K, V]) Rewind()`
*   `(it *Iterator[K, V]) Seek(key K)`

## Contributing

Contributions are welcome! Please feel free to submit a pull request or open an issue.

## License

This project is licensed under the MIT License - see the [LICENSE.md](LICENSE.md) file for details.
