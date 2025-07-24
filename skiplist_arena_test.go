package skiplist

import (
	"testing"
	"unsafe"
)

// TestArenaGrowth verifies that the memory arena grows automatically when it runs out of space.
// It creates an arena that can only hold 2 nodes, then inserts 3 to force a growth.
func TestArenaGrowth_WithFactor(t *testing.T) {
	// Calculate the approximate size of a single node struct.
	// The arena only allocates the node struct itself, not its internal slices.
	nodeSize := int(unsafe.Sizeof(node[int, int]{}))

	// Set an initial arena size that can hold exactly 2 nodes.
	initialSize := nodeSize * 2

	// Create a skiplist with the small arena and a growth factor.
	sl := New(
		WithArena[int, int](initialSize),
		WithArenaGrowthFactor[int, int](2.0), // Grow to 2x the size when full.
	)

	// Insert 3 items. The 3rd insertion should trigger arena growth.
	// If growth fails or doesn't happen, the 3rd Get() inside Insert() will panic.
	// This test passes if it doesn't panic and all items are present.
	sl.Insert(10, 10)
	sl.Insert(20, 20)
	sl.Insert(30, 30) // This should trigger growth.

	if sl.Len() != 3 {
		t.Fatalf("Expected length 3 after forcing arena growth, got %d", sl.Len())
	}

	// Verify all items are searchable.
	for _, key := range []int{10, 20, 30} {
		if _, ok := sl.Search(key); !ok {
			t.Errorf("Key %d not found after arena growth", key)
		}
	}
}

// TestArenaGrowth_WithBytes verifies that the memory arena grows by a fixed number of bytes.
func TestArenaGrowth_WithBytes(t *testing.T) {
	nodeSize := int(unsafe.Sizeof(node[int, int]{}))
	initialSize := nodeSize * 2
	growthBytes := nodeSize * 3 // Grow by enough for 3 more nodes

	sl := New(
		WithArena[int, int](initialSize),
		WithArenaGrowthBytes[int, int](growthBytes),
	)

	// Insert 3 items. The 3rd insertion should trigger growth.
	sl.Insert(10, 10)
	sl.Insert(20, 20)
	sl.Insert(30, 30) // Triggers growth. New chunk should be size `growthBytes`.

	if sl.Len() != 3 {
		t.Fatalf("Expected length 3, got %d", sl.Len())
	}

	// Now insert more items to test the new chunk.
	// The new chunk has space for 3 nodes. We can insert 2 more.
	sl.Insert(40, 40)
	sl.Insert(50, 50)

	if sl.Len() != 5 {
		t.Fatalf("Expected length 5 after filling grown chunk, got %d", sl.Len())
	}
}

// TestArenaGrowthThreshold verifies the proactive growth feature.
func TestArenaGrowthThreshold(t *testing.T) {
	nodeSize := int(unsafe.Sizeof(node[int, int]{}))

	// Initial size for 10 nodes.
	initialSize := nodeSize * 10
	// Set threshold at 85%. After 8 nodes (80%), the 9th node (90%) should trigger growth.
	threshold := 0.85

	sl := New(
		WithArena[int, int](initialSize),
		WithArenaGrowthThreshold[int, int](threshold),
	)

	// Insert enough items to cross the threshold and then some more.
	numItems := 12
	for i := 0; i < numItems; i++ {
		sl.Insert(i, i)
	}

	if sl.Len() != numItems {
		t.Fatalf("Expected length %d after crossing growth threshold, got %d", numItems, sl.Len())
	}
}

// TestArena_ResetAfterGrowth verifies that an arena, after being grown, can be
// reset via Clear() and reused correctly.
func TestArena_ResetAfterGrowth(t *testing.T) {
	nodeSize := int(unsafe.Sizeof(node[int, int]{}))
	initialSize := nodeSize * 2

	sl := New(
		WithArena[int, int](initialSize),
		WithArenaGrowthFactor[int, int](2.0),
	)

	// 1. Insert enough items to force growth.
	for i := 0; i < 5; i++ {
		sl.Insert(i, i)
	}
	if sl.Len() != 5 {
		t.Fatalf("Pre-clear: Expected length 5, got %d", sl.Len())
	}

	// 2. Clear the list, which should reset the arena.
	sl.Clear()
	if sl.Len() != 0 {
		t.Fatalf("Post-clear: Expected length 0, got %d", sl.Len())
	}

	// 3. Re-insert items. This should reuse the already allocated (and grown) arena chunks.
	// We should be able to insert more than the initial capacity without panicking.
	for i := 0; i < 5; i++ {
		sl.Insert(i, i)
	}
	if sl.Len() != 5 {
		t.Fatalf("Post-reuse: Expected length 5, got %d", sl.Len())
	}
}