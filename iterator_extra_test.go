package skiplist

import (
	"reflect"
	"testing"
	"time"
)

// Test that a RangeIterator holds the read lock until Close() is called,
// which blocks writers (Insert) until the iterator is closed.
func TestRangeIterator_CloseConcurrency(t *testing.T) {
	for _, setup := range getTestSetups[int, int]() {
		t.Run(setup.name, func(t *testing.T) {
			sl := setup.constructor(nil)
			sl.Insert(10, 10)
			sl.Insert(20, 20)

			maxInt := int(^uint(0) >> 1)
			it := sl.RangeIterator(0, maxInt)
			// ensure we release if the test fails early
			defer it.Close()

			// Advance once so iterator is active
			if !it.Next() {
				it.Close()
				t.Fatal("expected iterator to have elements")
			}

			done := make(chan struct{})

			// Writer goroutine: should block until iterator is closed
			go func() {
				sl.Insert(999, 999)
				close(done)
			}()

			// Give the goroutine a short time to attempt the insert; it should be blocked.
			select {
			case <-done:
				it.Close()
				t.Fatal("Insert completed while RangeIterator still held read-lock")
			case <-time.After(100 * time.Millisecond):
				// expected: still blocked
			}

			// Now close the iterator and expect the insert to complete.
			it.Close()

			select {
			case <-done:
				// success
			case <-time.After(1 * time.Second):
				t.Fatal("Insert did not complete after iterator.Close()")
			}
		})
	}
}

// Test that creating an iterator with WithEnd stops iteration at the inclusive bound.
func TestIterator_WithEnd(t *testing.T) {
	for _, setup := range getTestSetups[int, string]() {
		t.Run(setup.name, func(t *testing.T) {
			sl := setup.constructor(nil)
			sl.Insert(10, "a")
			sl.Insert(20, "b")
			sl.Insert(30, "c")
			sl.Insert(40, "d")

			it := sl.NewIterator(WithEnd[int, string](30))
			defer it.Close()

			var got []int
			for it.Next() {
				got = append(got, it.Key())
			}

			want := []int{10, 20, 30}
			if len(got) != len(want) {
				t.Fatalf("WithEnd: got %v, want %v", got, want)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("WithEnd mismatch at %d: got %v want %v", i, got, want)
				}
			}
		})
	}
}

// Test reverse iteration using WithReverse: should iterate from largest to smallest key.
func TestIterator_Reverse(t *testing.T) {
	for _, setup := range getTestSetups[int, int]() {
		t.Run(setup.name, func(t *testing.T) {
			sl := setup.constructor(nil)
			sl.Insert(10, 10)
			sl.Insert(20, 20)
			sl.Insert(30, 30)
			sl.Insert(40, 40)

			it := sl.NewIterator(WithReverse[int, int]())
			defer it.Close()

			var got []int
			for it.Next() {
				got = append(got, it.Key())
			}

			want := []int{40, 30, 20, 10}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Reverse iterator: got %v, want %v", got, want)
			}
		})
	}
}

// Test Prev on a reverse iterator: Prev should move forward (from first -> last).
func TestIterator_PrevWithReverse(t *testing.T) {
	for _, setup := range getTestSetups[int, int]() {
		t.Run(setup.name, func(t *testing.T) {
			sl := setup.constructor(nil)
			sl.Insert(10, 10)
			sl.Insert(20, 20)
			sl.Insert(30, 30)
			sl.Insert(40, 40)

			it := sl.NewIterator(WithReverse[int, int]())
			defer it.Close()

			// Position at the first element, then use Prev() which for reverse iterators
			// advances forward through the list. Include the current first element
			// before iterating with Prev().
			if !it.First() {
				t.Fatal("expected First() to succeed")
			}

			var got []int
			// include the first element
			got = append(got, it.Key())
			for it.Prev() {
				got = append(got, it.Key())
			}

			want := []int{10, 20, 30, 40}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Prev on reverse iterator: got %v, want %v", got, want)
			}
		})
	}
}

// Tests for reverse iteration combined with WithEnd bound semantics.
func TestIterator_ReverseWithEnd(t *testing.T) {
	for _, setup := range getTestSetups[int, int]() {
		t.Run(setup.name, func(t *testing.T) {
			sl := setup.constructor(nil)
			// populate
			sl.Insert(10, 10)
			sl.Insert(20, 20)
			sl.Insert(30, 30)
			sl.Insert(40, 40)
			sl.Insert(50, 50)

			t.Run("end trims tail", func(t *testing.T) {
				// WithEnd 35 should skip 50 and 40 and start at 30
				it := sl.NewIterator(WithReverse[int, int](), WithEnd[int, int](35))
				defer it.Close()

				var got []int
				for it.Next() {
					got = append(got, it.Key())
				}
				want := []int{30, 20, 10}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("ReverseWithEnd(trim): got %v want %v", got, want)
				}
			})

			t.Run("end larger than max returns all", func(t *testing.T) {
				it := sl.NewIterator(WithReverse[int, int](), WithEnd[int, int](100))
				defer it.Close()
				var got []int
				for it.Next() {
					got = append(got, it.Key())
				}
				want := []int{50, 40, 30, 20, 10}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("ReverseWithEnd(all): got %v want %v", got, want)
				}
			})

			t.Run("end smaller than min returns none", func(t *testing.T) {
				it := sl.NewIterator(WithReverse[int, int](), WithEnd[int, int](5))
				defer it.Close()
				if it.Next() {
					t.Fatalf("ReverseWithEnd(none): expected no elements but got first=%v", it.Key())
				}
			})
		})
	}
}
