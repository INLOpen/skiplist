# Go Generic Skiplist

[![Go](https://github.com/INLOpen/skiplist/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/INLOpen/skiplist/actions/workflows/go.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/INLOpen/skiplist)](https://goreportcard.com/report/github.com/INLOpen/skiplist) [![Go Reference](https://pkg.go.dev/badge/github.com/INLOpen/skiplist.svg)](https://pkg.go.dev/github.com/INLOpen/skiplist)

ไลบรารี Skiplist ที่เป็น Generic, ปลอดภัยสำหรับการทำงานพร้อมกัน (thread-safe), และมีประสิทธิภาพสูงสำหรับภาษา Go

ไลบรารีนี้มีโครงสร้างข้อมูล skiplist ที่ใช้งานง่าย มีประสิทธิภาพ และยืดหยุ่น ถูกออกแบบมาเพื่อเป็นทางเลือกที่ทรงพลังแทน `map` ของ Go ในสถานการณ์ที่คุณต้องการเข้าถึงข้อมูลที่เรียงลำดับ (sorted) โดยมี time complexity เฉลี่ยเป็น O(log n) สำหรับการค้นหา, เพิ่ม, และลบข้อมูล

Skiplist รักษาลำดับของข้อมูลโดยใช้ลำดับชั้นของ linked list หลายๆ ชั้น ชั้นล่างสุดจะมีข้อมูลหนาแน่น ในขณะที่ชั้นที่สูงขึ้นจะมีข้อมูลน้อยลง ทำหน้าที่เป็น "ช่องทางด่วน" เพื่อข้ามข้อมูลจำนวนมากไปได้อย่างรวดเร็ว ทำให้การค้นหาทำได้อย่างมีประสิทธิภาพ

```
Level 3: 1-------------------------------->9
          |                                |
Level 2: 1--------->4--------------------->9
          |          |                     |
Level 1: 1--->2----->4--------->6--------->9
          |   |      |          |         |
Data:    (1) (2)    (4)        (6)       (9)
```

## คุณสมบัติเด่น

*   **🚀 ประสิทธิภาพสูง**: Time complexity เฉลี่ย O(log n) สำหรับการดำเนินการหลัก
*   **🧠 ปรับแต่งการใช้หน่วยความจำ**:
	*   **`sync.Pool` (ค่าเริ่มต้น)**: นำโหนดกลับมาใช้ใหม่ (recycle) เพื่อลดการจองหน่วยความจำและลดภาระของ Garbage Collector (GC) เหมาะสำหรับงานที่มีการเพิ่ม/ลบข้อมูลบ่อย (high-churn)
	*   **⚡️ Memory Arena ที่ขยายขนาดได้ (ตัวเลือกเสริม)**: เพื่อประสิทธิภาพสูงสุดและลดผลกระทบต่อ GC สามารถเปิดใช้งาน memory arena allocator ที่สามารถขยายขนาดได้เองโดยอัตโนมัติ พร้อมกลยุทธ์การขยายที่ปรับแต่งได้
*   **⛓️ รองรับ Generic**: เป็น Generic อย่างสมบูรณ์ (`[K any, V any]`) ทำให้สามารถใช้ key และ value เป็น type ใดก็ได้
*   **🎛️ ปรับแต่งการเรียงลำดับได้**: รองรับฟังก์ชันเปรียบเทียบ (comparator) ที่กำหนดเอง ทำให้สามารถจัดการตรรกะการเรียงลำดับที่ซับซ้อนสำหรับ key type ใดๆ ก็ได้ (เช่น `structs`)
*   **🤝 ปลอดภัยสำหรับการทำงานพร้อมกัน (Thread-Safe)**: ทุกการดำเนินการปลอดภัยสำหรับการใช้งานพร้อมกันจากหลาย goroutine
*   **✨ API ครบครัน**: มีเมธอดที่ครอบคลุมการใช้งาน เช่น `RangeQuery`, `PopMin`, `PopMax`, `Predecessor`, `Successor` และอื่นๆ
*   **🚶 Iterator ที่มีความสามารถครบถ้วน**: มี iterator ที่ทรงพลัง สามารถเดินทางได้สองทิศทาง พร้อมเมธอด `Seek`, `Next`, `Prev`, `First`, `Last`, `Reset` และอื่นๆ เพื่อการเข้าถึงข้อมูลที่ยืดหยุ่น

## ทำไมถึงควรใช้ Skiplist นี้?

แม้ว่า `map` ของ Go จะยอดเยี่ยมสำหรับการเก็บข้อมูล key-value ทั่วไป แต่ `map` ไม่ได้รักษาลำดับของข้อมูล Skiplist นี้จึงเป็นตัวเลือกที่ดีกว่าในสถานการณ์ที่ข้อมูลที่เรียงลำดับแล้วมีความสำคัญ

| กรณีการใช้งาน | `map[K]V` | `sync.Map` | `Skiplist นี้` |
|---|---|---|---|
| **Key-Value ไม่เรียงลำดับ** | ✅ **ดีที่สุด** | ✅ (Concurrent) | (มี Overhead) |
| **การวนลูปแบบเรียงลำดับ** | ❌ (ไม่เรียงลำดับ) | ❌ (ไม่เรียงลำดับ) | ✅ **ดีที่สุด** |
| **หาค่า Key น้อยสุด/มากสุด** | ❌ (ต้องสแกนทั้งหมด) | ❌ (ต้องสแกนทั้งหมด) | ✅ **O(1)** |
| **ค้นหาข้อมูลเป็นช่วง (Range Query)** | ❌ (ต้องสแกนทั้งหมด) | ❌ (ต้องสแกนทั้งหมด) | ✅ **O(log n + k)** |
| **หาโหนดก่อนหน้า/ถัดไป** | ❌ (ไม่เรียงลำดับ) | ❌ (ไม่เรียงลำดับ) | ✅ **O(log n)** |
| **ควบคุม GC อย่างละเอียด** | ❌ | ❌ | ✅ (ผ่าน `sync.Pool` หรือ Arena) |

## กลยุทธ์การจัดสรรหน่วยความจำและประสิทธิภาพ

Skiplist นี้มีกลยุทธ์การจัดสรรหน่วยความจำ 2 แบบ ซึ่งแต่ละแบบมีลักษณะด้านประสิทธิภาพที่แตกต่างกัน คุณสามารถรัน benchmark ด้วยตนเองได้โดยใช้คำสั่ง `go test -bench=.`

*   **`sync.Pool` (ค่าเริ่มต้น)**: เป็นตัวเลือกมาตรฐานที่ประหยัดหน่วยความจำ เหมาะสำหรับงานที่มีการเพิ่ม/ลบข้อมูลบ่อย (high-churn) โดยการนำโหนดกลับมาใช้ใหม่ ซึ่งช่วยลดภาระของ Garbage Collector ได้อย่างมาก
*   **`Memory Arena` (ตัวเลือกเสริม)**: เป็นตัวเลือกที่เน้นปริมาณงานสูง (high-throughput) โดยการจองหน่วยความจำก้อนใหญ่ไว้ล่วงหน้า (เรียกว่า chunk) เมื่อ chunk เต็ม Arena จะสามารถ**ขยายขนาดได้เองโดยอัตโนมัติ**โดยการสร้าง chunk ใหม่ที่ใหญ่กว่าเดิม ซึ่งช่วยลด overhead ของ GC สำหรับการจัดสรรโหนดได้เกือบทั้งหมด ส่งผลให้ latency ต่ำและคาดเดาได้ง่ายขึ้นสำหรับการดำเนินการจำนวนมาก คุณสามารถกำหนดขนาดเริ่มต้น, สัดส่วนการขยาย, และแม้กระทั่ง threshold สำหรับการขยายล่วงหน้าได้

**สรุป**:
*   ใช้ **`sync.Pool` (ค่าเริ่มต้น)** สำหรับการใช้งานทั่วไปและสถานการณ์ที่มีการเพิ่ม/ลบข้อมูลบ่อย
*   ใช้ **`Memory Arena`** สำหรับ latency ที่ต่ำที่สุดเท่าที่จะเป็นไปได้ระหว่างการเพิ่ม/อ่านข้อมูลจำนวนมาก

## การติดตั้ง

```bash
$ go get github.com/INLOpen/skiplist
```

## การใช้งาน

### การใช้งานพื้นฐาน (สำหรับ Key ที่เรียงลำดับได้)

สำหรับ key type มาตรฐานที่รองรับการเรียงลำดับ (เช่น `int`, `string`) คุณสามารถใช้ constructor `New()` แบบง่ายได้

```go
package main

import (
	"fmt"
	"github.com/INLOpen/skiplist"
)

func main() {
	// สร้าง skiplist ใหม่สำหรับ key ประเภท int และ value ประเภท string
	// จะใช้ comparator เริ่มต้น (cmp.Compare) โดยอัตโนมัติ
	sl := skiplist.New[int, string]()

	sl.Insert(10, "สิบ")
	sl.Insert(20, "ยี่สิบ")
	sl.Insert(30, "สามสิบ")

	// ค้นหาข้อมูล
	node, ok := sl.Search(20)
	if ok {
		fmt.Printf("พบ key 20 มี value: %s\n", node.Value()) // "ยี่สิบ"
	}

	// วนลูปดูข้อมูลทั้งหมดตามลำดับ
	fmt.Println("ข้อมูลทั้งหมด:")
	sl.Range(func(key int, value string) bool {
		fmt.Printf("  %d: %s\n", key, value)
		return true // คืนค่า true เพื่อวนลูปต่อไป
	})

	// ดึงข้อมูลตัวสุดท้าย (มากที่สุด) ออก
	maxNode, ok := sl.PopMax()
	if ok {
		fmt.Printf("ดึงข้อมูลตัวสุดท้ายออก: %d -> %s\n", maxNode.Key(), maxNode.Value()) // 30 -> "สามสิบ"
	}

	fmt.Printf("จำนวนข้อมูลปัจจุบัน: %d\n", sl.Len()) // 2
}
```

### การใช้งาน Arena พื้นฐาน

สำหรับสถานการณ์ที่ต้องการ latency ต่ำที่สุดเท่าที่จะเป็นไปได้ เช่น การโหลดข้อมูลจำนวนมาก คุณสามารถเปิดใช้งาน memory arena ได้

```go
package main

import (
	"fmt"
	"github.com/INLOpen/skiplist"
)

func main() {
	// เพื่อประสิทธิภาพสูงสุด, สร้าง skiplist พร้อม memory arena ขนาด 128MB
	// เหมาะสำหรับสถานการณ์ที่คุณเพิ่มข้อมูลจำนวนมากและต้องการลด overhead ของ GC
	arenaOpt := skiplist.WithArena[int, string](128 * 1024 * 1024) // 128MB
	sl := skiplist.New[int, string](arenaOpt)

	// การดำเนินการเหมือนเดิม
	sl.Insert(1, "หนึ่ง")
	fmt.Println("จำนวนข้อมูลเมื่อใช้ Arena:", sl.Len())
}
```

### การใช้งาน Arena ขั้นสูง (พร้อมการขยายขนาดอัตโนมัติ)

Memory Arena สามารถกำหนดค่าให้เริ่มต้นด้วยขนาดเล็กและขยายขนาดได้เองโดยอัตโนมัติตามความจำเป็น

```go
package main

import (
	"fmt"
	"github.com/INLOpen/skiplist"
)

func main() {
	// กำหนดค่า arena ให้เริ่มต้นด้วยขนาดเล็กและขยายได้เอง
	// มีประโยชน์เมื่อเราไม่ทราบขนาดหน่วยความจำที่ต้องใช้ล่วงหน้า
	sl := skiplist.New[int, string](
		// เริ่มต้นด้วย arena ขนาดเล็ก 1KB
		skiplist.WithArena[int, string](1024),
		// เมื่อ arena เต็ม ให้ขยายขนาดเป็น 2 เท่าของขนาดเดิม
		skiplist.WithArenaGrowthFactor[int, string](2.0),
		// ขยายขนาดล่วงหน้าเมื่อ chunk ถูกใช้งานไปแล้ว 90% เพื่อลด fragmentation
		skiplist.WithArenaGrowthThreshold[int, string](0.9),
	)

	// เพิ่มข้อมูลมากกว่าที่ arena เริ่มต้นจะรับไหว เพื่อบังคับให้เกิดการขยายขนาด
	for i := 0; i < 1000; i++ {
		sl.Insert(i, fmt.Sprintf("value-%d", i))
	}

	fmt.Println("เพิ่มข้อมูล 1000 รายการสำเร็จด้วย arena ที่ขยายขนาดได้")
	fmt.Println("จำนวนข้อมูลสุดท้าย:", sl.Len())
}
```

### การใช้ Comparator ที่กำหนดเอง (สำหรับ Struct Keys)

หากต้องการใช้ struct ที่กำหนดเองเป็น key คุณต้องส่งฟังก์ชัน comparator ของคุณเองไปยัง `NewWithComparator`

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

// userComparator กำหนดวิธีการเรียงลำดับ User keys (เรียงตาม ID)
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
	// สร้าง skiplist พร้อมกับ comparator ที่กำหนดเอง
	sl := skiplist.NewWithComparator[User, string](userComparator)

	sl.Insert(User{ID: 2, Name: "Bob"}, "วิศวกร")
	sl.Insert(User{ID: 1, Name: "Alice"}, "ผู้จัดการ")

	// list จะถูกเรียงตาม User.ID
	minNode, ok := sl.Min()
	if ok {
		userKey := minNode.Key()
		fmt.Printf("User ที่น้อยที่สุดคือ: %s (ID: %d), ตำแหน่ง: %s\n", userKey.Name, userKey.ID, minNode.Value()) // "Alice (ID: 1), ตำแหน่ง: ผู้จัดการ"
	}
}
```

## API Reference

### Constructors
*   `New[K cmp.Ordered, V any](opts ...Option[K, V]) *SkipList[K, V]`
*   `NewWithComparator[K any, V any](compare Comparator[K], opts ...Option[K, V]) *SkipList[K, V]`

### Configuration Options
*   `WithArena[K, V](sizeInBytes int) Option[K, V]`
*   `WithArenaGrowthFactor[K, V](factor float64) Option[K, V])`
*   `WithArenaGrowthBytes[K, V](bytes int) Option[K, V]`
*   `WithArenaGrowthThreshold[K, V](threshold float64) Option[K, V]`

... (ส่วนที่เหลือของ API reference สามารถดูได้จากไฟล์ภาษาอังกฤษ)

## การมีส่วนร่วม

ยินดีรับทุกการมีส่วนร่วม! สามารถส่ง pull request หรือเปิด issue ได้เลย

## License

โปรเจกต์นี้อยู่ภายใต้ MIT License - ดูรายละเอียดได้ที่ไฟล์ LICENSE.md