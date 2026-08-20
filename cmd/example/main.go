// Command example demonstrates the go-containers library.
package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/wow-look-at-my/go-containers/concurrentbag"
	"github.com/wow-look-at-my/go-containers/concurrentlist"
	"github.com/wow-look-at-my/go-containers/concurrentmap"
	"github.com/wow-look-at-my/go-containers/concurrentstack"
	"github.com/wow-look-at-my/go-containers/set"
	"github.com/wow-look-at-my/go-containers/sortedmap"
)

func main() {
	s := set.Of(1, 2, 3)
	fmt.Println(s)

	m := sortedmap.New[string, int]()
	m.Put("alice", 1)
	m.Put("bob", 2)
	fmt.Println(m)

	concurrentMap()
	concurrentList()
	concurrentStack()
	concurrentBag()
	producerConsumer()
}

func concurrentMap() {
	hits := concurrentmap.New[string, int]()

	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			hits.AddOrUpdate("/index", 1, func(_ string, old int) int { return old + 1 })
		})
	}
	wg.Wait()

	count, _ := hits.Load("/index")
	fmt.Println("concurrentmap: /index counted", count, "times")
}

func concurrentList() {
	l := concurrentlist.New[int]()
	l.AppendRange(1, 2, 3, 4)

	first, _ := l.TryTake()
	fmt.Println("concurrentlist: oldest first, so", first, "then", l.Values())
}

func concurrentStack() {
	st := concurrentstack.New[string]()
	st.PushRange("first", "second", "third")

	top, _ := st.TryPop()
	fmt.Println("concurrentstack: newest first, so", top)
}

func concurrentBag() {
	bag := concurrentbag.New[int]()

	var wg sync.WaitGroup
	for worker := range 4 {
		wg.Go(func() { bag.AddRange(worker*10, worker*10+1) })
	}
	wg.Wait()

	fmt.Println("concurrentbag: no order, but nothing lost:", bag.Len(), "elements")
}

// A bounded blocking list is the producer and consumer pattern with a limit on
// the work in flight.
func producerConsumer() {
	ctx := context.Background()
	queue := concurrentlist.NewBlocking[int](concurrentlist.WithCapacity(8))

	var consumers sync.WaitGroup
	total := make([]int, 3)
	for c := range 3 {
		consumers.Go(func() {
			for v := range queue.Consume(ctx) {
				total[c] += v
			}
		})
	}

	for i := 1; i <= 100; i++ {
		if err := queue.Append(ctx, i); err != nil {
			fmt.Println("append stopped:", err)
			break
		}
	}
	queue.CompleteAdding()
	consumers.Wait()

	sum := 0
	for _, part := range total {
		sum += part
	}
	fmt.Println("blocking list: three consumers summed 1..100 to", sum)
}
