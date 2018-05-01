package deferred

import (
	"context"
	"net/http"
	"sync"
	"time"
)

type deferredHandler struct {
	*sync.Mutex
	handler http.Handler
}

func (h *deferredHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Lock()
	c := h.handler
	h.Unlock()
	c.ServeHTTP(w, r)
}

func newRepeater() (func(http.Handler), <-chan http.Handler) {
	receive, repeat := make(chan http.Handler), make(chan http.Handler)
	go func() {
		v := <-receive
		close(receive)
		for {
			repeat <- v
		}
	}()
	return func(next http.Handler) {
		receive <- next
	}, repeat
}

func failedHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "permanent error creating handler", http.StatusServiceUnavailable)
}

type options struct {
	notify       func(error)
	timeoutAfter time.Duration
	retryAfter   time.Duration
}

// Config is a function that mutates an options object
type Config func(options) options

// WithRetryAfter returns a Config that will set the given duration
// as the interval for retrying handler creation
func WithRetryAfter(v time.Duration) Config {
	return func(o options) options {
		o.retryAfter = v
		return o
	}
}

// WithNotify returns a Config that will use the given Notify func
func WithNotify(n func(error)) Config {
	return func(o options) options {
		o.notify = n
		return o
	}
}

// WithTimeoutAfter returns a Config that will use the given handler timeout value
func WithTimeoutAfter(v time.Duration) Config {
	return func(o options) options {
		o.timeoutAfter = v
		return o
	}
}

// default values populating options objects
const (
	DefaultRetryAfter   = time.Second * 10
	DefaultTimeoutAfter = time.Second * 15
)

// default values populating options objects
var (
	DefaultNotify = func(error) {}
)

// NewHandler creates a new http.Handler that will try to queue requests until the
// handler creation succeeded. On a failed creation attempt the passed notify function
// will be called with the error returned by `create`. In case the passed
// context is cancelled before a handler could be created, retrying will be
// terminated and the handler will permanently return 503.
func NewHandler(ctx context.Context, create func() (http.Handler, error), configs ...Config) http.Handler {
	settings := options{
		notify:       DefaultNotify,
		timeoutAfter: DefaultTimeoutAfter,
		retryAfter:   DefaultRetryAfter,
	}

	for _, c := range configs {
		settings = c(settings)
	}

	send, updateHandler := newRepeater()

	h := deferredHandler{
		Mutex: &sync.Mutex{},
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case h := <-updateHandler:
				h.ServeHTTP(w, r)
			case <-time.NewTimer(settings.timeoutAfter).C:
				http.Error(w, "timed out waiting for handler to be created and sent", http.StatusServiceUnavailable)
			}
		}),
	}

	go func() {
		next := <-updateHandler
		h.Lock()
		h.handler = next
		h.Unlock()
	}()

	go func() {
		schedule := make(chan time.Time)
		go func() {
			for t := time.Tick(settings.retryAfter); true; <-t {
				schedule <- time.Now()
			}
		}()
		for {
			select {
			case <-ctx.Done():
				send(http.HandlerFunc(failedHandler))
				return
			case <-schedule:
				next, err := create()
				if err == nil {
					send(next)
					return
				}
				settings.notify(err)
			}
		}
	}()

	return &h
}
