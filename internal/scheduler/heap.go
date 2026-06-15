package scheduler

import "container/heap"

// entryHeap implements heap.Interface for *entry, ordered by next run time (min-heap).
type entryHeap []*entry

func (h entryHeap) Len() int           { return len(h) }
func (h entryHeap) Less(i, j int) bool { return h[i].next.Before(h[j].next) }
func (h entryHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *entryHeap) Push(x any) {
	e := x.(*entry)
	e.index = len(*h)
	*h = append(*h, e)
}

func (h *entryHeap) Pop() any {
	old := *h
	n := len(old)
	e := old[n-1]
	old[n-1] = nil // prevent memory leak
	e.index = -1
	*h = old[:n-1]
	return e
}

// ensure the heap invariant is satisfied after creation
func newEntryHeap() entryHeap {
	h := make(entryHeap, 0)
	heap.Init(&h)
	return h
}
