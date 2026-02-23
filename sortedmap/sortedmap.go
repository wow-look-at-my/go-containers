// Package sortedmap provides a generic sorted map backed by a left-leaning
// red-black tree (LLRB). All keys are kept in sorted order at all times,
// giving O(log n) Put, Get, Delete, and ordered queries without ever
// needing to re-sort.
package sortedmap

import (
	"cmp"
	"fmt"
	"iter"
	"strings"
)

// node colors.
const (
	red   = true
	black = false
)

// node is an internal LLRB tree node.
type node[K, V any] struct {
	key   K
	value V
	left  *node[K, V]
	right *node[K, V]
	color bool
}

func isRed[K, V any](n *node[K, V]) bool {
	return n != nil && n.color == red
}

// SortedMap is an ordered key-value map that maintains keys in sorted order
// using a left-leaning red-black tree. It provides O(log n) time for Put,
// Get, Delete, Min, Max, Floor, and Ceiling.
//
// The zero value is not usable; create instances with [New] or [NewWithCompare].
type SortedMap[K, V any] struct {
	root *node[K, V]
	size int
	cmp  func(a, b K) int
}

// New creates an empty SortedMap that orders keys using their natural ordering.
func New[K cmp.Ordered, V any]() *SortedMap[K, V] {
	return &SortedMap[K, V]{cmp: cmp.Compare[K]}
}

// NewWithCompare creates an empty SortedMap that orders keys using the
// provided comparison function. The function must return a negative value
// when a < b, zero when a == b, and a positive value when a > b.
func NewWithCompare[K, V any](compare func(a, b K) int) *SortedMap[K, V] {
	return &SortedMap[K, V]{cmp: compare}
}

// ---------- basic operations ----------

// Put inserts or updates the value associated with key.
func (m *SortedMap[K, V]) Put(key K, value V) {
	m.root = m.put(m.root, key, value)
	m.root.color = black
}

// Get returns the value associated with key and true, or the zero value and
// false if the key is not present.
func (m *SortedMap[K, V]) Get(key K) (V, bool) {
	n := m.root
	for n != nil {
		switch c := m.cmp(key, n.key); {
		case c < 0:
			n = n.left
		case c > 0:
			n = n.right
		default:
			return n.value, true
		}
	}
	var zero V
	return zero, false
}

// Delete removes the key and its value from the map. It reports whether the
// key was present.
func (m *SortedMap[K, V]) Delete(key K) bool {
	if !m.Contains(key) {
		return false
	}
	if !isRed(m.root.left) && !isRed(m.root.right) {
		m.root.color = red
	}
	m.root = m.del(m.root, key)
	m.size--
	if m.root != nil {
		m.root.color = black
	}
	return true
}

// Contains reports whether the map contains the given key.
func (m *SortedMap[K, V]) Contains(key K) bool {
	_, ok := m.Get(key)
	return ok
}

// Len returns the number of key-value pairs in the map.
func (m *SortedMap[K, V]) Len() int { return m.size }

// IsEmpty reports whether the map contains no key-value pairs.
func (m *SortedMap[K, V]) IsEmpty() bool { return m.size == 0 }

// Clear removes all key-value pairs from the map.
func (m *SortedMap[K, V]) Clear() {
	m.root = nil
	m.size = 0
}

// ---------- ordered operations ----------

// Min returns the smallest key and its value. If the map is empty it returns
// zero values and false.
func (m *SortedMap[K, V]) Min() (K, V, bool) {
	if m.root == nil {
		var zk K
		var zv V
		return zk, zv, false
	}
	n := m.minNode(m.root)
	return n.key, n.value, true
}

// Max returns the largest key and its value. If the map is empty it returns
// zero values and false.
func (m *SortedMap[K, V]) Max() (K, V, bool) {
	if m.root == nil {
		var zk K
		var zv V
		return zk, zv, false
	}
	n := m.maxNode(m.root)
	return n.key, n.value, true
}

// Floor returns the largest key less than or equal to the given key, along
// with its value. If no such key exists it returns zero values and false.
func (m *SortedMap[K, V]) Floor(key K) (K, V, bool) {
	n := m.floor(m.root, key)
	if n == nil {
		var zk K
		var zv V
		return zk, zv, false
	}
	return n.key, n.value, true
}

// Ceiling returns the smallest key greater than or equal to the given key,
// along with its value. If no such key exists it returns zero values and false.
func (m *SortedMap[K, V]) Ceiling(key K) (K, V, bool) {
	n := m.ceiling(m.root, key)
	if n == nil {
		var zk K
		var zv V
		return zk, zv, false
	}
	return n.key, n.value, true
}

// ---------- iteration ----------

// All returns an iterator over all key-value pairs in ascending key order.
func (m *SortedMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		m.inOrder(m.root, yield)
	}
}

// Keys returns an iterator over all keys in ascending order.
func (m *SortedMap[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		m.inOrder(m.root, func(k K, _ V) bool {
			return yield(k)
		})
	}
}

// Values returns an iterator over all values in ascending key order.
func (m *SortedMap[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		m.inOrder(m.root, func(_ K, v V) bool {
			return yield(v)
		})
	}
}

// Backward returns an iterator over all key-value pairs in descending key order.
func (m *SortedMap[K, V]) Backward() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		m.reverseInOrder(m.root, yield)
	}
}

// Range returns an iterator over key-value pairs whose keys lie in [from, to]
// (inclusive) in ascending order.
func (m *SortedMap[K, V]) Range(from, to K) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		m.rangeInOrder(m.root, from, to, yield)
	}
}

// String returns a human-readable representation of the map in key order.
func (m *SortedMap[K, V]) String() string {
	var b strings.Builder
	b.WriteByte('{')
	first := true
	for k, v := range m.All() {
		if !first {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%v: %v", k, v)
		first = false
	}
	b.WriteByte('}')
	return b.String()
}

// ---------- internal LLRB operations ----------

func (m *SortedMap[K, V]) put(h *node[K, V], key K, value V) *node[K, V] {
	if h == nil {
		m.size++
		return &node[K, V]{key: key, value: value, color: red}
	}
	switch c := m.cmp(key, h.key); {
	case c < 0:
		h.left = m.put(h.left, key, value)
	case c > 0:
		h.right = m.put(h.right, key, value)
	default:
		h.value = value
	}
	return fixUp(h)
}

func (m *SortedMap[K, V]) del(h *node[K, V], key K) *node[K, V] {
	if m.cmp(key, h.key) < 0 {
		if !isRed(h.left) && !isRed(h.left.left) {
			h = moveRedLeft(h)
		}
		h.left = m.del(h.left, key)
	} else {
		if isRed(h.left) {
			h = rotateRight(h)
		}
		if m.cmp(key, h.key) == 0 && h.right == nil {
			return nil
		}
		if !isRed(h.right) && !isRed(h.right.left) {
			h = moveRedRight(h)
		}
		if m.cmp(key, h.key) == 0 {
			succ := m.minNode(h.right)
			h.key = succ.key
			h.value = succ.value
			h.right = m.deleteMin(h.right)
		} else {
			h.right = m.del(h.right, key)
		}
	}
	return fixUp(h)
}

func (m *SortedMap[K, V]) deleteMin(h *node[K, V]) *node[K, V] {
	if h.left == nil {
		return nil
	}
	if !isRed(h.left) && !isRed(h.left.left) {
		h = moveRedLeft(h)
	}
	h.left = m.deleteMin(h.left)
	return fixUp(h)
}

func (m *SortedMap[K, V]) minNode(n *node[K, V]) *node[K, V] {
	for n.left != nil {
		n = n.left
	}
	return n
}

func (m *SortedMap[K, V]) maxNode(n *node[K, V]) *node[K, V] {
	for n.right != nil {
		n = n.right
	}
	return n
}

func (m *SortedMap[K, V]) floor(n *node[K, V], key K) *node[K, V] {
	if n == nil {
		return nil
	}
	c := m.cmp(key, n.key)
	if c == 0 {
		return n
	}
	if c < 0 {
		return m.floor(n.left, key)
	}
	if t := m.floor(n.right, key); t != nil {
		return t
	}
	return n
}

func (m *SortedMap[K, V]) ceiling(n *node[K, V], key K) *node[K, V] {
	if n == nil {
		return nil
	}
	c := m.cmp(key, n.key)
	if c == 0 {
		return n
	}
	if c > 0 {
		return m.ceiling(n.right, key)
	}
	if t := m.ceiling(n.left, key); t != nil {
		return t
	}
	return n
}

// ---------- traversal helpers ----------

func (m *SortedMap[K, V]) inOrder(n *node[K, V], yield func(K, V) bool) bool {
	if n == nil {
		return true
	}
	return m.inOrder(n.left, yield) &&
		yield(n.key, n.value) &&
		m.inOrder(n.right, yield)
}

func (m *SortedMap[K, V]) reverseInOrder(n *node[K, V], yield func(K, V) bool) bool {
	if n == nil {
		return true
	}
	return m.reverseInOrder(n.right, yield) &&
		yield(n.key, n.value) &&
		m.reverseInOrder(n.left, yield)
}

func (m *SortedMap[K, V]) rangeInOrder(n *node[K, V], from, to K, yield func(K, V) bool) bool {
	if n == nil {
		return true
	}
	cmpFrom := m.cmp(from, n.key)
	cmpTo := m.cmp(to, n.key)
	if cmpFrom < 0 {
		if !m.rangeInOrder(n.left, from, to, yield) {
			return false
		}
	}
	if cmpFrom <= 0 && cmpTo >= 0 {
		if !yield(n.key, n.value) {
			return false
		}
	}
	if cmpTo > 0 {
		if !m.rangeInOrder(n.right, from, to, yield) {
			return false
		}
	}
	return true
}

// ---------- red-black tree balancing ----------

func rotateLeft[K, V any](h *node[K, V]) *node[K, V] {
	x := h.right
	h.right = x.left
	x.left = h
	x.color = h.color
	h.color = red
	return x
}

func rotateRight[K, V any](h *node[K, V]) *node[K, V] {
	x := h.left
	h.left = x.right
	x.right = h
	x.color = h.color
	h.color = red
	return x
}

func flipColors[K, V any](h *node[K, V]) {
	h.color = !h.color
	h.left.color = !h.left.color
	h.right.color = !h.right.color
}

func fixUp[K, V any](h *node[K, V]) *node[K, V] {
	if isRed(h.right) && !isRed(h.left) {
		h = rotateLeft(h)
	}
	if isRed(h.left) && isRed(h.left.left) {
		h = rotateRight(h)
	}
	if isRed(h.left) && isRed(h.right) {
		flipColors(h)
	}
	return h
}

func moveRedLeft[K, V any](h *node[K, V]) *node[K, V] {
	flipColors(h)
	if isRed(h.right.left) {
		h.right = rotateRight(h.right)
		h = rotateLeft(h)
		flipColors(h)
	}
	return h
}

func moveRedRight[K, V any](h *node[K, V]) *node[K, V] {
	flipColors(h)
	if isRed(h.left.left) {
		h = rotateRight(h)
		flipColors(h)
	}
	return h
}
