package nntp

import "testing"

func TestQueue(t *testing.T) {
	q := NewQueue()
	if !q.Empty() {
		t.Errorf("NewQueue returns a Queue that is not Empty.")
	}

	q.Enqueue(1)

	if q.Empty() {
		t.Errorf("Enqueue makes a Queue that is not Empty.")
	}

	if v := q.Dequeue().(int); v != 1 {
		t.Errorf("Dequeue return %d instead of 1.", v)
	}

	if !q.Empty() {
		t.Errorf("Dequeue didn't make a Queue Empty which contained only one element.")
	}

	q.Enqueue(1)
	q.Enqueue(2)
	q.Enqueue(3)

	if v := q.Dequeue().(int); v != 1 {
		t.Errorf("Dequeue return %d instead of 1.", v)
	}

	if v := q.Dequeue().(int); v != 2 {
		t.Errorf("Dequeue return %d instead of 2.", v)
	}

	if v := q.Dequeue().(int); v != 3 {
		t.Errorf("Dequeue return %d instead of 3.", v)
	}

	if !q.Empty() {
		t.Errorf("Dequeue didn't make a Queue Empty which contained only one element.")
	}
}
