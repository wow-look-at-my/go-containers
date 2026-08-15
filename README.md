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

## Development

Build and test with [go-toolchain](https://github.com/wow-look-at-my/go-toolchain):

```bash
go-toolchain
```

`cmd/example` is a runnable tour of the packages.
