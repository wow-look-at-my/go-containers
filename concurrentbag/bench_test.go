package concurrentbag

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

// Every benchmark runs Bag against a mutex-guarded slice and a buffered
// channel -- what a Go caller reaches for without a bag. Add benchmarks run
// against an arena one goroutine replaces every arenaOps ops, so a growing
// container never exhausts memory and no implementation pays for a drain.

const (
	// arenaOps is how many ops one goroutine runs before replacing the arena.
	arenaOps = 4096
	// bulkSize is the AddRange benchmark's per-operation batch length.
	bulkSize = 64
	// refillBatch keeps a take loop's container populated instead of empty.
	refillBatch = 1024
	// steady is what the mixed benchmark starts with, so takes hit a populated container.
	steady = 4096
)

// parallelisms is the b.SetParallelism sweep, in multiples of GOMAXPROCS.
var parallelisms = []int{1, 4, 16}

// sink keeps every result alive so the compiler cannot delete the work.
var sink atomic.Int64

// newBag builds the bag under test. New takes options, so it does not match
// the build signature the loops want.
func newBag() *Bag[int] {
	return New[int]()
}

// filledBag builds a bag that holds prefill values.
func filledBag(prefill int) func() *Bag[int] {
	return func() *Bag[int] {
		bag := New[int]()
		for i := range prefill {
			bag.Add(i)
		}
		return bag
	}
}

// mutexBag is a slice behind a mutex, the hand-rolled bag.
type mutexBag struct {
	mu    sync.Mutex
	items []int
}

func newMutexBag(capacity, prefill int) *mutexBag {
	m := &mutexBag{items: make([]int, 0, capacity)}
	for i := range prefill {
		m.items = append(m.items, i)
	}
	return m
}

func (m *mutexBag) add(v int) {
	m.mu.Lock()
	m.items = append(m.items, v)
	m.mu.Unlock()
}

// addRange is the loop of appends the caller writes in place of AddRange.
func (m *mutexBag) addRange(values []int) {
	m.mu.Lock()
	m.items = append(m.items, values...)
	m.mu.Unlock()
}

func (m *mutexBag) tryTake() (int, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.items) == 0 {
		return 0, false
	}
	v := m.items[len(m.items)-1]
	m.items = m.items[:len(m.items)-1]
	return v, true
}

func (m *mutexBag) length() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.items)
}

// chanBag is a buffered channel used as a bag. It is bounded, so an add fails
// when the buffer is full.
type chanBag struct {
	ch chan int
}

func newChanBag(capacity, prefill int) *chanBag {
	c := &chanBag{ch: make(chan int, capacity)}
	for i := range prefill {
		c.ch <- i
	}
	return c
}

func (c *chanBag) add(v int) {
	select {
	case c.ch <- v:
	default:
	}
}

func (c *chanBag) addRange(values []int) {
	for _, v := range values {
		c.add(v)
	}
}

func (c *chanBag) tryTake() (int, bool) {
	select {
	case v := <-c.ch:
		return v, true
	default:
		return 0, false
	}
}

func (c *chanBag) length() int {
	return len(c.ch)
}

// impl names one implementation of a workload.
type impl struct {
	name string
	// run gets the goroutine count, so a bounded impl can size its buffer.
	run func(b *testing.B, goroutines int)
}

// sweep runs every implementation at every parallelism.
func sweep(b *testing.B, impls ...impl) {
	b.Helper()
	for _, mult := range parallelisms {
		goroutines := mult * runtime.GOMAXPROCS(0)
		for _, im := range impls {
			b.Run(fmt.Sprintf("p=%d/%s", mult, im.name), func(b *testing.B) {
				b.SetParallelism(mult)
				b.ReportAllocs()
				im.run(b, goroutines)
			})
		}
	}
}

// arena holds the container under test and replaces it on demand.
type arena[C any] struct {
	cur   atomic.Pointer[C]
	mu    sync.Mutex
	build func() *C
}

func newArena[C any](build func() *C) *arena[C] {
	a := &arena[C]{build: build}
	a.cur.Store(build())
	return a
}

// replace swaps in a fresh container, but only if old is still current. The
// identity check keeps a burst of callers to one allocation.
func (a *arena[C]) replace(old *C) {
	a.mu.Lock()
	if a.cur.Load() == old {
		a.cur.Store(a.build())
	}
	a.mu.Unlock()
}

// addLoop measures an add-only workload against a replaced arena.
func addLoop[C any](b *testing.B, build func() *C, add func(*C, int)) {
	a := newArena(build)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i, since := 0, 0
		for pb.Next() {
			c := a.cur.Load()
			add(c, i)
			i++
			since++
			if since == arenaOps {
				since = 0
				a.replace(c)
			}
		}
	})
	sink.Add(1)
}

// takeLoop measures a take-only workload against a prefilled arena. A
// container never gives more than it got, so a take that finds it empty puts
// refillBatch values back. Every measured take therefore also pays for one
// add. Subtract the add benchmark to isolate the take itself.
func takeLoop[C any](b *testing.B, build func() *C, take func(*C) (int, bool), add func(*C, int)) {
	a := newArena(build)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var local int64
		for pb.Next() {
			c := a.cur.Load()
			v, ok := take(c)
			if !ok {
				for i := range refillBatch {
					add(c, i)
				}
				continue
			}
			local += int64(v)
		}
		sink.Add(local)
	})
}

// mixedLoop measures one add plus one take per operation. That workload holds
// the container at a steady size, which is what a bag is built for. The
// container starts with steady values, so the take reads a populated
// container and not an empty one.
func mixedLoop[C any](b *testing.B, build func() *C, add func(*C, int), take func(*C) (int, bool)) {
	a := newArena(build)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var local int64
		i := 0
		for pb.Next() {
			c := a.cur.Load()
			add(c, i)
			i++
			if v, ok := take(c); ok {
				local += int64(v)
			}
		}
		sink.Add(local)
	})
}

// lenLoop measures Len on a container that holds prefill values.
func lenLoop[C any](b *testing.B, build func() *C, length func(*C) int) {
	c := build()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var local int64
		for pb.Next() {
			local += int64(length(c))
		}
		sink.Add(local)
	})
}

// ---------- parallel add ----------

func BenchmarkCompareAddParallel(b *testing.B) {
	sweep(b,
		impl{"bag", func(b *testing.B, _ int) {
			addLoop(b, newBag, (*Bag[int]).Add)
		}},
		impl{"mutexslice", func(b *testing.B, g int) {
			addLoop(b, func() *mutexBag { return newMutexBag(2*arenaOps*g, 0) }, (*mutexBag).add)
		}},
		// The channel gets room for a whole arena generation, so a send never
		// fails inside one. A full buffer would measure the failure path.
		impl{"chan", func(b *testing.B, g int) {
			addLoop(b, func() *chanBag { return newChanBag(2*arenaOps*g, 0) }, (*chanBag).add)
		}},
	)
}

// ---------- parallel take ----------

func BenchmarkCompareTakeParallel(b *testing.B) {
	sweep(b,
		impl{"bag", func(b *testing.B, g int) {
			takeLoop(b, filledBag(arenaOps*g), (*Bag[int]).TryTake, (*Bag[int]).Add)
		}},
		impl{"mutexslice", func(b *testing.B, g int) {
			takeLoop(b, func() *mutexBag { return newMutexBag(arenaOps*g, arenaOps*g) },
				(*mutexBag).tryTake, (*mutexBag).add)
		}},
		impl{"chan", func(b *testing.B, g int) {
			takeLoop(b, func() *chanBag { return newChanBag(2*arenaOps*g, arenaOps*g) },
				(*chanBag).tryTake, (*chanBag).add)
		}},
	)
}

// ---------- parallel add and take ----------

func BenchmarkCompareAddTakeParallel(b *testing.B) {
	sweep(b,
		impl{"bag", func(b *testing.B, _ int) {
			mixedLoop(b, filledBag(steady), (*Bag[int]).Add, (*Bag[int]).TryTake)
		}},
		impl{"mutexslice", func(b *testing.B, g int) {
			mixedLoop(b, func() *mutexBag { return newMutexBag(2*arenaOps*g, steady) },
				(*mutexBag).add, (*mutexBag).tryTake)
		}},
		impl{"chan", func(b *testing.B, g int) {
			mixedLoop(b, func() *chanBag { return newChanBag(2*arenaOps*g, steady) },
				(*chanBag).add, (*chanBag).tryTake)
		}},
	)
}

// ---------- bulk add ----------

// BenchmarkCompareAddRangeParallel adds a batch per operation. Bag.AddRange
// links the whole batch with one compare-and-swap. The other two run a loop.
func BenchmarkCompareAddRangeParallel(b *testing.B) {
	batch := make([]int, bulkSize)
	for i := range batch {
		batch[i] = i
	}
	sweep(b,
		impl{"bag", func(b *testing.B, _ int) {
			bulkLoop(b, newBag, func(c *Bag[int]) { c.AddRange(batch...) })
		}},
		impl{"mutexslice", func(b *testing.B, g int) {
			bulkLoop(b, func() *mutexBag { return newMutexBag(2*arenaOps*g, 0) },
				func(c *mutexBag) { c.addRange(batch) })
		}},
		impl{"chan", func(b *testing.B, g int) {
			bulkLoop(b, func() *chanBag { return newChanBag(2*arenaOps*g, 0) },
				func(c *chanBag) { c.addRange(batch) })
		}},
	)
}

// bulkLoop is addLoop for a batch. It replaces the arena bulkSize times more
// often, because one operation adds bulkSize values.
func bulkLoop[C any](b *testing.B, build func() *C, addBatch func(*C)) {
	a := newArena(build)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		since := 0
		for pb.Next() {
			c := a.cur.Load()
			addBatch(c)
			since++
			if since == arenaOps/bulkSize {
				since = 0
				a.replace(c)
			}
		}
	})
	sink.Add(1)
}

// ---------- Len ----------

func BenchmarkCompareLenParallel(b *testing.B) {
	const prefill = 10000
	sweep(b,
		impl{"bag", func(b *testing.B, _ int) {
			lenLoop(b, filledBag(prefill), (*Bag[int]).Len)
		}},
		impl{"mutexslice", func(b *testing.B, _ int) {
			lenLoop(b, func() *mutexBag { return newMutexBag(prefill, prefill) }, (*mutexBag).length)
		}},
		impl{"chan", func(b *testing.B, _ int) {
			lenLoop(b, func() *chanBag { return newChanBag(prefill, prefill) }, (*chanBag).length)
		}},
	)
}

// ---------- one goroutine ----------

// serial runs one implementation without b.RunParallel.
func serial(b *testing.B, impls ...impl) {
	b.Helper()
	for _, im := range impls {
		b.Run(im.name, func(b *testing.B) {
			b.ReportAllocs()
			im.run(b, 1)
		})
	}
}

// addSerial measures the uncontended add path.
func addSerial[C any](b *testing.B, build func() *C, add func(*C, int)) {
	c := build()
	b.ResetTimer()
	for i := range b.N {
		add(c, i)
		if i%arenaOps == arenaOps-1 {
			c = build()
		}
	}
	sink.Add(1)
}

func BenchmarkCompareAddSerial(b *testing.B) {
	serial(b,
		impl{"bag", func(b *testing.B, _ int) { addSerial(b, newBag, (*Bag[int]).Add) }},
		impl{"mutexslice", func(b *testing.B, _ int) {
			addSerial(b, func() *mutexBag { return newMutexBag(arenaOps, 0) }, (*mutexBag).add)
		}},
		impl{"chan", func(b *testing.B, _ int) {
			addSerial(b, func() *chanBag { return newChanBag(arenaOps, 0) }, (*chanBag).add)
		}},
	)
}

// takeSerial measures the uncontended take path. The refill runs with the
// timer stopped, so the number is a take and nothing else.
func takeSerial[C any](b *testing.B, build func() *C, take func(*C) (int, bool)) {
	c := build()
	var local int64
	b.ResetTimer()
	for range b.N {
		v, ok := take(c)
		if !ok {
			b.StopTimer()
			c = build()
			b.StartTimer()
			continue
		}
		local += int64(v)
	}
	sink.Add(local)
}

func BenchmarkCompareTakeSerial(b *testing.B) {
	const prefill = arenaOps * 8
	serial(b,
		impl{"bag", func(b *testing.B, _ int) {
			takeSerial(b, filledBag(prefill), (*Bag[int]).TryTake)
		}},
		impl{"mutexslice", func(b *testing.B, _ int) {
			takeSerial(b, func() *mutexBag { return newMutexBag(prefill, prefill) }, (*mutexBag).tryTake)
		}},
		impl{"chan", func(b *testing.B, _ int) {
			takeSerial(b, func() *chanBag { return newChanBag(prefill, prefill) }, (*chanBag).tryTake)
		}},
	)
}
