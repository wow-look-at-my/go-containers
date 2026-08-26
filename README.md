# go-containers

Generic container types for Go: a set, a sorted map, a weak-referenced event,
and a family of concurrent collections — a sharded map, a lock-free ordered
list, a lock-free stack and bag, and a blocking twin of each. Pure Go, one
dependency (testify, tests only).

Every concurrent type keeps its synchronization inside itself. No caller ever
takes a lock, and no method hands one back.

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
When subscribers must return a value, use `event.ResultEvent[T, R]` and
`*func(T) (R, error)` instead.

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

var queried event.ResultEvent[ClickArgs, int]
// results, err := queried.Invoke(ClickArgs{X: 1, Y: 2})
```

Invoke calls every live callback even when one returns an error, and joins the
errors. A collected callback is skipped and dropped. ResultEvent.Invoke also
returns each callback's value.

The argument type must embed `event.Args`. That rules out a bare `int` or
`string` argument, so an event can gain a field later without breaking every
subscriber.

## concurrentmap

`concurrentmap.Map[K, V]` shards its keys across independently locked
partitions, after .NET's `ConcurrentDictionary`. Two goroutines that touch
different shards never wait for each other.

```go
hits := concurrentmap.New[string, int]()
hits.AddOrUpdate("/index", 1, func(_ string, old int) int { return old + 1 })
cfg, _ := hits.LoadOrCompute("/slow", expensiveToBuild)  // runs once, ever
```

`LoadOrCompute`, `AddOrUpdate` and `Compute` run your function under the shard
lock, so it runs **exactly once** and the whole operation is atomic. That is
the one thing `sync.Map` and `ConcurrentDictionary` cannot promise. The rule it
buys: the function must not call back into the same map.

Also: `Store`, `Load`, `TryAdd`, `LoadOrStore`, `Delete`, `LoadAndDelete`,
`Len`, `Clear`, the `All`/`Keys`/`Values` iterators, `ToMap`, and the
package-level `CompareAndSwap` and `CompareAndDelete`. `New` is required; the
zero value panics with a message that says so.

## concurrentset

`concurrentset.Set[T]` is an unordered collection of unique elements, safe for
concurrent use. It is a thin wrapper over `concurrentmap.Map[T, struct{}]`,
so it inherits that type's sharded locking rather than reimplementing it.

```go
seen := concurrentset.New[string]()
if seen.Add(id) {
    // id was not seen before
}
```

Also: `AddRange`, `Remove`, `Contains`, `Len`, `IsEmpty`, `Clear`, the `All`
iterator, and `Values`. `New` takes the same options as `concurrentmap.New`.
Every operation is one `concurrentmap` call; see the Performance section below
for the benchmark that checks the wrapper costs nothing over a hand-rolled set.

## concurrentlist, concurrentstack, concurrentbag

Three lock-free collections that differ only in the order they give back.

```go
l := concurrentlist.New[int]()   // first in, first out
st := concurrentstack.New[int]() // last in, first out
bag := concurrentbag.New[int]()  // no order, fastest under contention

l.AppendRange(1, 2, 3)           // one atomic add for the whole run
v, ok := l.TryTake()             // 1
```

Each has bulk methods that cost one atomic operation and one allocation for a
whole run, `TryPeek`, an `All` iterator and `Values` that remove nothing, and
an O(1) `Len`. None of them uses a mutex anywhere.

The bag is the one to reach for when order does not matter: it shards, so its
adds and takes scale where the others contend on one head.

## The blocking collections

Each of the three has a blocking twin with the same name and the same
vocabulary: `concurrentlist.BlockingList`, `concurrentstack.BlockingStack`,
`concurrentbag.BlockingBag`. They are .NET's `BlockingCollection`, one per
order instead of one wrapper around all of them.

```go
queue := concurrentlist.NewBlocking[Job](concurrentlist.WithCapacity(64))

go func() {
	for job := range queue.Consume(ctx) { work(job) }  // blocks until done
}()

queue.Append(ctx, job)   // blocks while full
queue.CompleteAdding()   // consumers drain, then Consume returns
```

A bounded collection makes an add wait while it is full, which stops producers
running away from consumers. Every wait ends on a context, so no caller can be
stuck. After `CompleteAdding` the consumers drain what is left and then see
`ErrCompleted` — the same error value in all three packages, so one consumer
handles any of them.

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

The concurrent figures below are **medians of five runs** on one 4-vCPU
sandbox, at `--benchtime 200ms`. Single runs of the same code moved by up to 3x
here, so anything under about 1.5x is noise. `p=N` means N x GOMAXPROCS
goroutines. The comparisons are against a mutex around a map or a slice,
`sync.Map`, and a buffered channel.

**concurrentbag** is the clearest win, and the reason is that it is the only
one of the four that shards a lock-free structure. Against a mutex-guarded
slice it takes 2.1-2.5x less time on parallel adds (31/37/41ns vs 65/94/91ns at
p=1/4/16) and 2.1-2.9x less on parallel takes (60/66/86ns vs 148/189/180ns). It
also beats a buffered channel on both. `Len` is a sum of padded counters: 3.5ns
against 57-92ns, and it does not degrade with parallelism.

**concurrentstack** wins the contended push (62/66/66ns, flat, against
62/91/85ns) and `Len` (0.6ns against 80-96ns), and ties on pops. It **loses**
the mixed push-and-pop workload badly: 213-222ns against 144-178ns for the
mutex, and 74-77ns for a channel. One allocation per push, plus pointer chasing
through a cold chain, is more than a single uncontended mutex costs.

**concurrentlist** wins the contended append (122-223ns against 240-247ns), and
`Len` (2.2ns against 17.8ns). On a round trip of one append and one take it is
flat at about 202ns, where the mutex queue runs 143ns at p=1 and 184-194ns
under real contention — so the mutex wins uncontended and the two converge as
contention rises. A buffered channel wins that workload outright at 84-92ns,
and it is the right answer whenever a fixed-size FIFO handoff is all you need.
The list earns its place on the things a channel cannot do at all: read without
removing, bulk transfer, no fixed bound, and `TryTake` that never blocks.

**concurrentmap** wins contended writes against a mutex-guarded map by 2.6-2.8x
(115-120ns against 164-326ns) and contended `LoadOrStore` by 4-5x (19-21ns
against 86-92ns), and it never allocates where `sync.Map` allocates two per
store. `Len` costs 217ns against 84.7µs for counting a `sync.Map` by hand. It
**loses** reads to `sync.Map` (28-30ns against 7.5-8.1ns), which is an atomic
load of an immutable map and hard to beat, and it loses a 90/10 read-write
mix to both alternatives. If a workload is read-mostly over a stable key set,
use `sync.Map`. Reach for this when writes contend, when `Len` is on a hot
path, or when you need the exactly-once `Compute` family.

**concurrentset** wraps `concurrentmap.Map[T, struct{}]` rather than sharding
and locking again from scratch, and a `DirectSet` benchmark baseline (same
sharding, one `set.Set[T]` per shard instead of `concurrentmap`'s generic
`Map`) shows the wrapper is not just as fast -- for `Add` it is faster: 67/79/80ns
for `Set` against 104/128/129ns for `DirectSet` at p=1/4/16, both against a
mutex-guarded `set.Set` at 102/110/113ns. The gap is semantics, not sharding:
`concurrentmap.Map.TryAdd` checks presence before writing, while `set.Set.Add`
always writes and compares lengths after, so a hot, mostly-already-present key
set pays for a map mutation `TryAdd` skips. `Contains` has no such asymmetry
and the two are within noise (65/60/60ns against 60/61/60ns), both well ahead
of the mutex baseline (81/105/111ns).

The methodology, including the two rules that keep a concurrent benchmark from
lying, is in [docs/concurrency.md](docs/concurrency.md).

## Development

Build and test with [go-toolchain](https://github.com/wow-look-at-my/go-toolchain):

```bash
go-toolchain                        # build, vet, test, benchmark
GOFLAGS=-race go-toolchain --cgo    # the same, under the race detector
```

The second command is not optional for a change to a concurrent package: a
plain `go-toolchain` run does not enable the race detector. CI runs both.

`cmd/example` is a runnable tour of the packages. The comparison suite lives in
the `*/bench_test.go` files; the single-implementation benchmarks live at the
foot of each package's `*_test.go`.
