package main

import (
	"fmt"
	"runtime"
	"time"

	"math/rand/v2"

	"github.com/INLOpen/skiplist"
)

func main() {
	const N = 200000

	// prepare keys
	r := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	keys := make([]int, N)
	for i := 0; i < N; i++ {
		keys[i] = r.Int()
	}

	configs := []struct {
		name string
		opts []skiplist.Option[int, int]
	}{
		{"Arena-default-1MB", []skiplist.Option[int, int]{skiplist.WithArena[int, int](1 << 20)}},
		{"Arena-factor-2-1KB", []skiplist.Option[int, int]{skiplist.WithArena[int, int](1 << 10), skiplist.WithArenaGrowthFactor[int, int](2.0)}},
		{"Arena-bytes-64KB-1KB", []skiplist.Option[int, int]{skiplist.WithArena[int, int](1 << 10), skiplist.WithArenaGrowthBytes[int, int](64 * 1024)}},
		{"Arena-threshold-0.9-factor-2-1KB", []skiplist.Option[int, int]{skiplist.WithArena[int, int](1 << 10), skiplist.WithArenaGrowthFactor[int, int](2.0), skiplist.WithArenaGrowthThreshold[int, int](0.9)}},
	}

	fmt.Printf("Running lightweight arena insert microbench (N=%d)\n", N)

	for _, cfg := range configs {
		runtime.GC()
		time.Sleep(50 * time.Millisecond)
		fmt.Printf("\nConfig: %s\n", cfg.name)

		sl := skiplist.New[int, int](cfg.opts...)

		var msBefore, msAfter runtime.MemStats
		runtime.ReadMemStats(&msBefore)
		start := time.Now()

		for i := 0; i < N; i++ {
			sl.Insert(keys[i], i)
		}

		dur := time.Since(start)
		runtime.ReadMemStats(&msAfter)

		nsPerOp := float64(dur.Nanoseconds()) / float64(N)
		allocDiff := int64(msAfter.TotalAlloc) - int64(msBefore.TotalAlloc)

		fmt.Printf("Duration: %s, ns/op: %.1f, TotalAlloc diff: %d bytes, Len: %d\n", dur, nsPerOp, allocDiff, sl.Len())
	}
}
