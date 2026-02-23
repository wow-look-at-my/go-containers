// Command example demonstrates the go-containers library.
package main

import (
	"fmt"

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
}
