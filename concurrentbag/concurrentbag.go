// Package concurrentbag provides Bag, a thread-safe unordered collection that
// keeps duplicates. It is modelled on .NET's ConcurrentBag<T> and it is
// lock-free: every operation is a compare-and-swap loop over a Treiber stack.
//
// Go gives library code no goroutine identity and no P identity. A bag
// therefore cannot copy .NET's thread-local affinity, where the same thread
// adds to and takes from its own list. Each operation picks a shard at random
// instead, and a take that finds its shard empty steals from the other shards.
package concurrentbag

import (
	"iter"
	"math/bits"
	"math/rand/v2"
	"runtime"
	"sync/atomic"
)

const (
	// cacheLine is the padding unit that keeps two shards apart. 64 bytes is
	// the line size on amd64 and on arm64.
	cacheLine = 64

	// shardsPerProc scales the shard count above GOMAXPROCS, so two goroutines
	// that pick at random rarely land on one shard.
	shardsPerProc = 4

	// minShards is the floor on the shard count. A machine with few cores can
	// still run many goroutines, and two shards do not spread them.
	minShards = 8
)

// node is one element of a shard chain. A node is never recycled, and next is
// immutable once the node is reachable from a shard top.
type node[T any] struct {
	value T
	next  *node[T]
}

// shard is a Treiber stack plus its own length counter. The padding gives each
// shard its own cache line, so a counter never shares a line with the next
// shard's counter. The test asserts the size.
type shard[T any] struct {
	top atomic.Pointer[node[T]]
	n   atomic.Int64
	_   [cacheLine - 16]byte
}

// pushChain links a locally built chain onto the shard with one
// compare-and-swap per attempt. head is the new top and tail is the far end.
// The caller must own the whole chain: no other goroutine may reach it yet.
func (s *shard[T]) pushChain(head, tail *node[T], count int64) {
	// The counter rises before the chain is visible. A take can only follow a
	// successful CAS, so the counter never drops below the true length.
	s.n.Add(count)
	for {
		top := s.top.Load()
		tail.next = top
		if s.top.CompareAndSwap(top, head) {
			return
		}
	}
}

// pop removes the top node and reports whether it got one.
func (s *shard[T]) pop() (T, bool) {
	for {
		top := s.top.Load()
		if top == nil {
			var zero T
			return zero, false
		}
		// ABA cannot happen here. The bag never recycles a node, and this
		// goroutine holds a live reference to top, so the collector cannot
		// give that address to a new node while the CAS runs. A free list or
		// a sync.Pool of nodes breaks this. Never add one.
		next := top.next
		if s.top.CompareAndSwap(top, next) {
			s.n.Add(-1)
			return top.value, true
		}
	}
}

// Bag is a thread-safe unordered collection of values of type T. It keeps
// duplicates. The zero value is NOT usable: create a bag with [New].
//
// Every method is safe for concurrent use. No method blocks and no method
// takes a lock.
type Bag[T any] struct {
	shards []shard[T]
	// mask selects a shard from a random word. len(shards) is a power of two,
	// so mask is len(shards)-1.
	mask uint64
}

// config holds the settings the options write.
type config struct {
	concurrency int
}

// Option configures a bag at construction. See [WithConcurrency].
type Option func(*config)

// WithConcurrency sets the number of goroutines the bag expects. The bag uses
// this number in place of GOMAXPROCS to size its shards. A value below one has
// no effect.
func WithConcurrency(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.concurrency = n
		}
	}
}

// New creates an empty bag. The shard count is a power of two at or above the
// concurrency level, with a floor of 8.
func New[T any](opts ...Option) *Bag[T] {
	c := config{concurrency: runtime.GOMAXPROCS(0)}
	for _, opt := range opts {
		opt(&c)
	}
	count := max(c.concurrency*shardsPerProc, minShards)
	count = 1 << bits.Len(uint(count-1))
	return &Bag[T]{
		shards: make([]shard[T], count),
		mask:   uint64(count - 1),
	}
}

// pick returns the index of a random shard. rand.Uint64 reads a per-P
// generator inside the runtime, so a pick costs no shared atomic.
func (b *Bag[T]) pick() int {
	return int(rand.Uint64() & b.mask)
}

// Add puts value into the bag.
func (b *Bag[T]) Add(value T) {
	n := &node[T]{value: value}
	b.shards[b.pick()].pushChain(n, n, 1)
}

// TryAdd puts value into the bag and reports true. The bag is unbounded, so an
// add never fails. The method exists for a caller that programs against a
// try-add store contract.
func (b *Bag[T]) TryAdd(value T) bool {
	b.Add(value)
	return true
}

// AddRange puts every value into the bag. It builds the whole chain first and
// links it into one shard with a single compare-and-swap per attempt.
func (b *Bag[T]) AddRange(values ...T) {
	if len(values) == 0 {
		return
	}
	// One allocation holds the whole range. The cost is that the last node of
	// a range keeps the memory of the whole range, until a take removes that
	// one too.
	nodes := make([]node[T], len(values))
	for i, v := range values {
		nodes[i].value = v
		if i > 0 {
			nodes[i].next = &nodes[i-1]
		}
	}
	head, tail := &nodes[len(nodes)-1], &nodes[0]
	b.shards[b.pick()].pushChain(head, tail, int64(len(values)))
}

// TryTake removes one value and reports whether it got one. It tries its own
// shard first, then it steals from the others. It reports false only after it
// saw every shard empty.
func (b *Bag[T]) TryTake() (T, bool) {
	start := b.pick()
	for i := range b.shards {
		if v, ok := b.shards[(start+i)&int(b.mask)].pop(); ok {
			return v, true
		}
	}
	var zero T
	return zero, false
}

// TryTakeRange removes up to len(buf) values into buf. It returns the number
// of values it wrote.
func (b *Bag[T]) TryTakeRange(buf []T) int {
	got := 0
	if len(buf) == 0 {
		return got
	}
	start := b.pick()
	for i := range b.shards {
		s := &b.shards[(start+i)&int(b.mask)]
		for got < len(buf) {
			v, ok := s.pop()
			if !ok {
				break
			}
			buf[got] = v
			got++
		}
		if got == len(buf) {
			break
		}
	}
	return got
}

// TryPeek reads one value without removal and reports whether it got one.
// Another goroutine can take that value before the caller acts on it.
func (b *Bag[T]) TryPeek() (T, bool) {
	start := b.pick()
	for i := range b.shards {
		if n := b.shards[(start+i)&int(b.mask)].top.Load(); n != nil {
			return n.value, true
		}
	}
	var zero T
	return zero, false
}

// Len returns the number of values in the bag. The count is exact when no
// other goroutine touches the bag. Under concurrent use it can count an add
// that is not linked yet, but it never undercounts and never falls below zero.
func (b *Bag[T]) Len() int {
	var total int64
	for i := range b.shards {
		total += b.shards[i].n.Load()
	}
	return int(total)
}

// IsEmpty reports whether the bag holds no values. It carries the same
// accuracy as [Bag.Len].
func (b *Bag[T]) IsEmpty() bool {
	return b.Len() == 0
}

// Clear removes every value the bag holds. A value another goroutine adds
// during the clear can survive it, because Clear detaches one shard at a time.
func (b *Bag[T]) Clear() {
	for i := range b.shards {
		s := &b.shards[i]
		var count int64
		for n := s.top.Swap(nil); n != nil; n = n.next {
			count++
		}
		s.n.Add(-count)
	}
}

// Values returns every value in the bag in indeterminate order. See [Bag.All]
// for what the result is a snapshot of.
func (b *Bag[T]) Values() []T {
	out := make([]T, 0, b.Len())
	for v := range b.All() {
		out = append(out, v)
	}
	return out
}

// All returns an iterator over every value in the bag, in indeterminate order.
//
// The walk reads one shard chain at a time. The result is therefore a snapshot
// per shard, not a snapshot of the whole bag at one instant. A node another
// goroutine takes during the walk stays valid, because the collector keeps it
// alive while the walk holds it, and because its next pointer never changes.
// So the walk can report a value that is already gone.
func (b *Bag[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for i := range b.shards {
			for n := b.shards[i].top.Load(); n != nil; n = n.next {
				if !yield(n.value) {
					return
				}
			}
		}
	}
}
