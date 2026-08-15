# go-containers

Generic container types for Go: a set, a sorted map, and a weak-referenced
event. Pure Go, one dependency (testify, tests only).

```bash
go get github.com/wow-look-at-my/go-containers
```

## set

`set.Set[T]` holds unique `comparable` elements. Reach for it instead of
`map[T]struct{}` or a `map[T]bool` whose values are all `true` — those spell a
set out by hand and give the reader a value to wonder about. go-toolchain's
`mapset` vet check reports both shapes and points here.

```go
import "github.com/wow-look-at-my/go-containers/set"

byteEncodingNames := set.Of("ISO-8859-1", "LATIN1", "L1")
byteEncodingNames.Contains("LATIN1")            // true
byteEncodingNames.Union(otherNames).Len()       // set algebra, no loops
```

The zero value is an empty set ready to use. Add, AddRange, Remove, Contains,
ContainsAll, ContainsAny, Len, IsEmpty, Clear, Clone, Values, All (an
iterator), and the algebraic operations: Union, Intersection, Difference,
SymmetricDifference, the subset and superset predicates, Equal, IsDisjoint,
and the in-place AddSet, RemoveSet, RetainAll. A set marshals to a JSON array
and back.

## sortedmap

`sortedmap.SortedMap[K, V]` keeps its keys in sorted order at all times, so an
ordered walk never sorts. A left-leaning red-black tree gives O(log n) Put,
Get, Delete, Min, Max, Floor, and Ceiling.

```go
import "github.com/wow-look-at-my/go-containers/sortedmap"

m := sortedmap.New[string, int]()          // natural order
m.Put("alice", 1)
for k, v := range m.All() { … }            // in key order
for k, v := range m.Range("a", "m") { … }  // half-open key range
```

`NewWithCompare` takes your own comparison function for a key type that has no
natural order. Iterators: All, Keys, Values, Backward, Range.

## event

`event.Event[T]` dispatches to callbacks it holds as **weak** references, so a
subscriber that goes away does not keep itself alive through the event. Keep
your own `*func(T) error` alive for as long as you want the subscription.

```go
import "github.com/wow-look-at-my/go-containers/event"

type ClickArgs struct {
	event.Args
	X, Y int
}

var clicked event.Event[ClickArgs]
cb := func(a ClickArgs) error { … }
clicked.Subscribe(&cb)        // keep cb alive yourself
err := clicked.Invoke(ClickArgs{X: 1, Y: 2})
```

Invoke calls every live callback even when one returns an error, and joins the
errors. A collected callback is skipped and dropped.

The argument type must embed `event.Args`. That rules out a bare `int` or
`string` argument, so an event can gain a field later without breaking every
subscriber.

## Performance

There are two benchmark suites, and both run on every build. `Benchmark*`
measures one implementation on its own. `BenchmarkCompare*` measures the
library type against the code a caller writes instead, on the same workload.
Reproduce with `go-toolchain` in the repo root; the figures below come from the
comparison suite on one linux/amd64 run, so read the ratios, not the digits.

**set** matches a hand-rolled `map[T]struct{}` everywhere the two do the same
work: membership, Add, Remove, iteration, Clone, Values, and the algebra are
all within a few percent, because they are the same loops. Two differences are
real, and both favour the library. `Add` reports whether the element was new
and still costs one hash (18.9ns vs 18.6ns to fill, 9.6ns vs 14.7ns to re-add,
where the hand-rolled version has to look up and then assign). `Intersection`
iterates the smaller side, so a 10-element set against a 100,000-element one
takes 698ns where the obvious loop over the receiver takes 1.36ms.

**sortedmap** is the trade a tree always is. A plain map wins every point
operation (Get 9ns vs 124ns at 10,000 keys). The tree wins the moment order is
asked for, because the map has to sort first: ordered iteration 90µs vs 724µs,
a 100-key Range 1.07µs vs 722µs, Min 4ns vs 90µs, Floor 140ns vs 129µs. A
sorted slice reads faster than the tree and writes far slower (Put 743ns vs
263ns, Delete 4.5µs vs 875ns at 10,000 keys).

**event** dispatches without allocating at any subscriber count. One
subscriber -- the common shape -- costs 52ns and returns its error unwrapped;
past that, dispatch runs about 10x a mutex-guarded callback slice (1.89µs vs
176ns at 100 subscribers). That gap is the weak reference: reading each
callback through a `weak.Pointer` is what keeps a dead subscriber from leaking
through the event, and it is paid per callback per dispatch. The slice holds
its subscribers alive forever instead, and dispatches while still holding its
lock, so a callback that subscribes there deadlocks.

## Development

Build and test with [go-toolchain](https://github.com/wow-look-at-my/go-toolchain):

```bash
go-toolchain
```

`cmd/example` is a runnable tour of the packages. The comparison suite lives in
the `*/bench_test.go` files; the single-implementation benchmarks live at the
foot of each package's `*_test.go`.
