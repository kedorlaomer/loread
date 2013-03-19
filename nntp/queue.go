package nntp

// Implementation of linked-list queues
type Queue struct {
	first, last *queueNode
}

type queueNode struct {
	data interface{} // payload of specific type
	Next *queueNode
}

// Create an empty queue.
func NewQueue() *Queue {
	return &Queue{}
}

// Is q empty?
func (q *Queue) Empty() bool {
	return q.first == nil && q.last == nil
}

// Add c to the end of q.
func (q *Queue) Enqueue(c interface{}) {
	node := &queueNode{
		data: c,
	}

	if q.first == nil {
		q.first = node
		q.last = node
	} else {
		q.last.Next = node
		q.last = node
	}
}

func (q *Queue) Dequeue() interface{} {
	if q.first == nil {
		panic("Dequeue on empty queue.")
	}

	rv := q.first.data
	q.first = q.first.Next

	if q.first == nil {
		q.last = nil
	}

	return rv
}
