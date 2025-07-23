package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof" // Import for side effects: registers pprof handlers
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/INLOpen/skiplist"
)

func main() {
	// เปิด pprof endpoint ผ่าน HTTP server
	// ซึ่งจะทำงานใน goroutine แยกต่างหาก
	go func() {
		fmt.Println("Starting pprof server on http://localhost:6060/debug/pprof/")
		// http.ListenAndServe จะ block การทำงาน, ถ้า return แสดงว่ามี error
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			log.Fatalf("pprof server failed: %v", err)
		}
	}()

	// รอให้ server เริ่มทำงานสักครู่
	time.Sleep(100 * time.Millisecond)

	// อ่านค่า numItems และ allocatorType จาก command-line
	numItems, allocatorType := parseArgs()

	fmt.Println("Starting skiplist insertion workload...")
	fmt.Printf(" - Items to insert: %d\n", numItems)
	fmt.Printf(" - Allocator: %s\n", allocatorType)

	// สร้าง skiplist โดยสามารถเลือก allocator ผ่าน command-line argument ได้
	sl := createSkipList(numItems, allocatorType)

	// เพิ่มข้อมูลจำนวนมากเพื่อสร้างภาระงานให้ CPU
	for i := 0; i < numItems; i++ {
		sl.Insert(i, i)
	}

	fmt.Printf("Finished inserting %d items. List length: %d\n", numItems, sl.Len())
	fmt.Println("Program is keeping alive for profiling. Press Ctrl+C to exit.")

	// ทำให้โปรแกรมทำงานค้างไว้เพื่อให้เราสามารถเชื่อมต่อ pprof server ได้
	// การ select จาก channel ที่ไม่มีวันได้รับข้อมูลเป็นวิธีที่นิยมใช้
	select {}
}

// parseArgs แยกวิเคราะห์ arguments จาก command-line
// Usage: go run ./cmd/profiler [allocator_type] [num_items]
// Example: go run ./cmd/profiler arena 5000000
func parseArgs() (numItems int, allocatorType string) {
	// Default values
	allocatorType = "pool"
	numItems = 2_000_000

	if len(os.Args) > 1 {
		allocatorType = os.Args[1]
	}
	if len(os.Args) > 2 {
		if n, err := strconv.Atoi(os.Args[2]); err == nil {
			numItems = n
		}
	}
	return numItems, allocatorType
}

// createSkipList สร้าง skiplist ตาม allocator ที่ระบุใน command-line
func createSkipList(numItems int, allocatorType string) *skiplist.SkipList[int, int] {
	if allocatorType == "arena" {
		// ประเมินขนาดของ Arena: ประมาณ 400 bytes ต่อโหนด
		arenaSize := numItems * 400
		fmt.Printf("Using Arena allocator with size %d MB\n", arenaSize/(1024*1024))
		return skiplist.New(skiplist.WithArena[int,int](arenaSize))
	}

	fmt.Println("Using Pool allocator (default)")
	runtime.GC() // สั่งให้ GC ทำงานเพื่อดู memory ก่อนเริ่ม
	return skiplist.New[int, int]()
}