package queue

import "github.com/acme/coderunner/internal/models"

// Queue provides a buffered channel for Job messages.
type Queue struct {
    ch chan models.Job
}

// New returns a new queue of given size.
func New(size int) *Queue {
    return &Queue{ch: make(chan models.Job, size)}
}

// Enqueue adds a job to the queue.
func (q *Queue) Enqueue(j models.Job) {
    q.ch <- j
}

// Dequeue removes a job from the queue, blocking if empty.
func (q *Queue) Dequeue() models.Job {
    return <-q.ch
}

// Chan returns a receive-only channel for iteration.
func (q *Queue) Chan() <-chan models.Job { return q.ch } 