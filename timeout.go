package aether

import (
	"context"
	"sync"
	"time"
)

// deadlineCtx is a request context with a deadline that can be removed while
// the request is running. Unlike context.WithTimeout, clearing it keeps
// propagating the parent's cancellation (for example a client disconnect), so
// long-lived responses such as Server-Sent Events can opt out of the request
// timeout without losing that signal.
type deadlineCtx struct {
	context.Context

	mu         sync.Mutex
	done       chan struct{}
	err        error
	deadline   time.Time
	timer      *time.Timer
	stopParent func() bool
}

func newDeadlineCtx(parent context.Context, d time.Duration) *deadlineCtx {
	c := &deadlineCtx{
		Context:  parent,
		done:     make(chan struct{}),
		deadline: time.Now().Add(d),
	}
	// Hold the lock so a timer or parent that fires immediately waits until
	// both fields are set.
	c.mu.Lock()
	defer c.mu.Unlock()
	c.timer = time.AfterFunc(d, func() { c.cancel(context.DeadlineExceeded) })
	c.stopParent = context.AfterFunc(parent, func() { c.cancel(parent.Err()) })
	return c
}

// Deadline returns the request deadline while it is active, or the parent's
// deadline once it has been cleared.
func (c *deadlineCtx) Deadline() (time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.deadline.IsZero() {
		return c.deadline, true
	}
	return c.Context.Deadline()
}

// Done returns a channel closed when the deadline passes, the parent is
// canceled or the request ends.
func (c *deadlineCtx) Done() <-chan struct{} {
	return c.done
}

// Err returns why the context was canceled, or nil while it is active.
func (c *deadlineCtx) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *deadlineCtx) cancel(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return
	}
	c.err = err
	c.timer.Stop()
	close(c.done)
}

// clear removes the deadline. It has no effect once the context is canceled.
func (c *deadlineCtx) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err == nil && c.timer.Stop() {
		c.deadline = time.Time{}
	}
}

// finish cancels the context at the end of the request and releases the
// parent watcher.
func (c *deadlineCtx) finish() {
	c.stopParent()
	c.cancel(context.Canceled)
}
