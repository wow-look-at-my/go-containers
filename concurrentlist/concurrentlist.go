// Package concurrentlist provides List, a lock-free ordered collection.
//
// A List keeps its elements in one total order: takes return the elements in
// the order the appends reserved them, across every goroutine. This is the
// ordered counterpart to concurrentbag, which keeps no order at all.
//
// The storage is a chain of segments, and a segment is one contiguous array of
// slots. Contiguous storage is what makes the list fast: an append reserves one
// slot with one atomic add, and the elements of a segment share cache lines.
// There is no mutex on any path, and no operation can block another one.
package concurrentlist

import (
	"iter"
	"runtime"
	"sync"
	"sync/atomic"
)

const (
	// initialSegmentLen keeps an empty list cheap. A list that stays small
	// never allocates a second segment.
	initialSegmentLen = 32
	// maxSegmentLen bounds the memory that one taken element can hold. A
	// segment is released as a whole, so a large segment delays that.
	maxSegmentLen = 4096
	// spinsBeforeYield bounds the wait for a producer that reserved a slot
	// and did not fill it yet. That wait ends after one store.
	spinsBeforeYield = 24
)

// slot holds one element and the flag that publishes it.
//
// The producer writes value and then stores ready. A consumer reads ready and
// then reads value. That order is what makes the plain value field safe.
type slot[T any] struct {
	ready atomic.Bool
	value T
}

// waitReady returns when the producer of this slot has published its value.
func (s *slot[T]) waitReady() {
	for i := 0; !s.ready.Load(); i++ {
		if i >= spinsBeforeYield {
			runtime.Gosched()
			i = 0
		}
	}
}

// segment is one contiguous array of slots, and one link in the chain.
//
// head and tail are positions inside this segment, and both only rise. tail can
// rise above len(slots): a producer that reserves a position past the end
// leaves it unused and moves to the next segment.
type segment[T any] struct {
	slots []slot[T]
	head  atomic.Uint64
	tail  atomic.Uint64
	next  atomic.Pointer[segment[T]]
}

func newSegment[T any](n int) *segment[T] {
	return &segment[T]{slots: make([]slot[T], n)}
}

// filled reports the number of positions in this segment that a producer
// reserved. It never exceeds the length of the segment.
func (s *segment[T]) filled() uint64 {
	n := s.tail.Load()
	if size := uint64(len(s.slots)); n > size {
		return size
	}
	return n
}

// List is a lock-free, ordered collection with first-in-first-out takes.
// The zero value is an empty list ready to use.
//
// Every method is safe for concurrent use. The caller never locks anything.
type List[T any] struct {
	head   atomic.Pointer[segment[T]]
	tail   atomic.Pointer[segment[T]]
	count  atomic.Int64
	initMu sync.Mutex
}

// New creates an empty list. The zero List is equally usable; New exists for
// callers that want a pointer in one expression.
func New[T any]() *List[T] {
	return &List[T]{}
}

// tailSegment returns the segment that accepts appends, and creates the first
// segment when the list is still empty.
func (l *List[T]) tailSegment() *segment[T] {
	if s := l.tail.Load(); s != nil {
		return s
	}
	l.initMu.Lock()
	defer l.initMu.Unlock()
	if s := l.tail.Load(); s != nil {
		return s
	}
	s := newSegment[T](initialSegmentLen)
	l.head.Store(s)
	l.tail.Store(s)
	return s
}

// grow links a further segment after full and moves the list tail onto it.
// Several goroutines can call this at the same time, and one of them wins.
func (l *List[T]) grow(full *segment[T]) {
	next := full.next.Load()
	if next == nil {
		size := len(full.slots) * 2
		if size > maxSegmentLen {
			size = maxSegmentLen
		}
		fresh := newSegment[T](size)
		if full.next.CompareAndSwap(nil, fresh) {
			next = fresh
		} else {
			next = full.next.Load()
		}
	}
	l.tail.CompareAndSwap(full, next)
}

// publish fills the reserved slot and makes it visible to takers.
//
// The count rises before the ready flag, so a taker that sees the element
// always sees a count that already includes it. Len can therefore never report
// a negative number.
func (l *List[T]) publish(s *slot[T], value T) {
	s.value = value
	l.count.Add(1)
	s.ready.Store(true)
}

// Append adds value to the end of the list. It never blocks.
func (l *List[T]) Append(value T) {
	for {
		s := l.tailSegment()
		pos := s.tail.Add(1) - 1
		if pos < uint64(len(s.slots)) {
			l.publish(&s.slots[pos], value)
			return
		}
		l.grow(s)
	}
}

// AppendRange adds every value to the end of the list, in the given order.
//
// One atomic add reserves a whole run of slots, so a bulk append costs far
// fewer atomic operations than the same number of Append calls. The run stays
// contiguous unless it crosses the end of a segment.
func (l *List[T]) AppendRange(values ...T) {
	for len(values) > 0 {
		s := l.tailSegment()
		size := uint64(len(s.slots))
		start := s.tail.Add(uint64(len(values))) - uint64(len(values))
		if start >= size {
			l.grow(s)
			continue
		}
		n := size - start
		if n > uint64(len(values)) {
			n = uint64(len(values))
		}
		// One count for the whole run. The count still rises before any
		// ready flag, which is what keeps Len from going negative.
		for i := uint64(0); i < n; i++ {
			s.slots[start+i].value = values[i]
		}
		l.count.Add(int64(n))
		for i := uint64(0); i < n; i++ {
			s.slots[start+i].ready.Store(true)
		}
		values = values[n:]
		if len(values) > 0 {
			l.grow(s)
		}
	}
}

// TryAdd adds value to the end of the list and always reports true. It is
// Append under the name that the Blocking types drive a store through.
func (l *List[T]) TryAdd(value T) bool {
	l.Append(value)
	return true
}

// TryTake removes and returns the oldest element. It reports false when the
// list is empty.
func (l *List[T]) TryTake() (T, bool) {
	for {
		s := l.head.Load()
		if s == nil {
			var zero T
			return zero, false
		}
		pos := s.head.Load()
		if pos < s.filled() {
			if !s.head.CompareAndSwap(pos, pos+1) {
				continue
			}
			el := &s.slots[pos]
			el.waitReady()
			l.count.Add(-1)
			return el.value, true
		}
		if pos < uint64(len(s.slots)) {
			var zero T
			return zero, false
		}
		next := s.next.Load()
		if next == nil {
			var zero T
			return zero, false
		}
		l.head.CompareAndSwap(s, next)
	}
}

// TryTakeRange removes up to len(buf) elements into buf, oldest first, and
// returns how many it wrote.
//
// One compare-and-swap claims a whole run of slots, so a bulk take costs far
// fewer atomic operations than the same number of TryTake calls.
func (l *List[T]) TryTakeRange(buf []T) int {
	n := 0
	for n < len(buf) {
		s := l.head.Load()
		if s == nil {
			break
		}
		pos := s.head.Load()
		limit := s.filled()
		if pos >= limit {
			if pos < uint64(len(s.slots)) {
				break
			}
			next := s.next.Load()
			if next == nil {
				break
			}
			l.head.CompareAndSwap(s, next)
			continue
		}
		take := limit - pos
		if room := uint64(len(buf) - n); take > room {
			take = room
		}
		if !s.head.CompareAndSwap(pos, pos+take) {
			continue
		}
		for i := uint64(0); i < take; i++ {
			el := &s.slots[pos+i]
			el.waitReady()
			buf[n] = el.value
			n++
		}
		l.count.Add(-int64(take))
	}
	return n
}

// TryPeek returns the oldest element and leaves it in the list. It reports
// false when the list is empty.
func (l *List[T]) TryPeek() (T, bool) {
	for {
		s := l.head.Load()
		if s == nil {
			var zero T
			return zero, false
		}
		pos := s.head.Load()
		if pos < s.filled() {
			el := &s.slots[pos]
			el.waitReady()
			return el.value, true
		}
		if pos < uint64(len(s.slots)) {
			var zero T
			return zero, false
		}
		next := s.next.Load()
		if next == nil {
			var zero T
			return zero, false
		}
		l.head.CompareAndSwap(s, next)
	}
}

// Len returns the number of elements in the list.
//
// The result is a reading, not a lock: another goroutine can change the length
// before the caller acts on it.
func (l *List[T]) Len() int {
	n := l.count.Load()
	if n < 0 {
		return 0
	}
	return int(n)
}

// IsEmpty reports whether the list holds no elements.
func (l *List[T]) IsEmpty() bool {
	return l.Len() == 0
}

// Clear removes elements until the list looks empty.
//
// Clear takes the elements one by one, so an append that runs at the same time
// is never lost. It is not one atomic step: a list that other goroutines still
// fill can be non-empty when Clear returns.
func (l *List[T]) Clear() {
	for {
		if _, ok := l.TryTake(); !ok {
			return
		}
	}
}

// All returns an iterator over the elements, oldest first, and removes none of
// them.
//
// The walk reads the segments as it goes. An element it yields can already be
// taken by another goroutine, and an element appended during the walk can
// appear. Use it for a report, never for a decision that must be exact.
func (l *List[T]) All() iter.Seq[T] {
	return func(yield func(T) bool) {
		for s := l.head.Load(); s != nil; s = s.next.Load() {
			limit := s.filled()
			for pos := s.head.Load(); pos < limit; pos++ {
				el := &s.slots[pos]
				if !el.ready.Load() {
					return
				}
				if !yield(el.value) {
					return
				}
			}
		}
	}
}

// Values returns the elements as a slice, oldest first, and removes none of
// them. It has the same best-effort reading as All.
func (l *List[T]) Values() []T {
	out := make([]T, 0, l.Len())
	for v := range l.All() {
		out = append(out, v)
	}
	return out
}
