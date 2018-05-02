package deferred

import (
	"context"
	"net/http"
	"sync"
	"time"
)

type deferred struct {
	sync.Mutex
	handler http.Handler
}

func (h *deferred) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.Lock()
	c := h.handler
	h.Unlock()
	c.ServeHTTP(w, r)
}

// default values populating options objects
const (
	DefaultRetryAfter   = time.Second * 10
	DefaultTimeoutAfter = time.Second * 15
)

// default values populating options objects
var (
	DefaultNotify        = func(error) {}
	DefaultFailedHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "permanent error creating handler", http.StatusServiceUnavailable)
	})
)

type options struct {
	notify                   func(error)
	failedHandler            http.Handler
	timeoutAfter, retryAfter time.Duration
}

func newOptions(configs ...Config) options {
	o := options{
		notify:        DefaultNotify,
		failedHandler: DefaultFailedHandler,
		retryAfter:    DefaultRetryAfter,
		timeoutAfter:  DefaultTimeoutAfter,
	}
	for _, c := range configs {
		o = c(o)
	}
	return o
}

// Config is a function that returns an updated options object
// when being passed another
type Config func(options) options

// WithRetryAfter returns a Config that will ensure the given duration
// is used as the interval for retrying handler creation
func WithRetryAfter(v time.Duration) Config {
	return func(o options) options {
		o.retryAfter = v
		return o
	}
}

// WithFailedHandler returns a Config that will ensure the given handler
// will be used when the creation has permanently failed
func WithFailedHandler(h http.Handler) Config {
	return func(o options) options {
		o.failedHandler = h
		return o
	}
}

// WithNotify returns a Config that will ensure the given Notify func
// is called when handler creation fails
func WithNotify(n func(error)) Config {
	return func(o options) options {
		o.notify = n
		return o
	}
}

// WithTimeoutAfter returns a Config that will ensure the pending handler
// will timeout after the given duration
func WithTimeoutAfter(v time.Duration) Config {
	return func(o options) options {
		o.timeoutAfter = v
		return o
	}
}

func awaitHandler() (func(http.Handler), <-chan http.Handler) {
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

// NewHandler returns a new http.Handler that will try to queue requests until
// handler creation succeeded. On a failed creation attempt the notify function
// will be called with the error returned by `create` if it is configured.
// In case the passed context is cancelled before a handler could be created,
// retrying will be terminated and the handler will permanently return 503 or
// use the behavior of a passed `FailedHandler` Config.
func NewHandler(ctx context.Context, create func() (http.Handler, error), configs ...Config) http.Handler {
	opts := newOptions(configs...)
	resolve, nextHandler := awaitHandler()

	h := deferred{
		handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case handler := <-nextHandler:
				handler.ServeHTTP(w, r)
			case <-time.After(opts.timeoutAfter):
				http.Error(w, "timed out waiting for handler to be created and sent", http.StatusServiceUnavailable)
			}
		}),
	}

	go func() {
		handler := <-nextHandler
		h.Lock()
		h.handler = handler
		h.Unlock()
	}()

	go func() {
		schedule := time.NewTimer(0)
		defer schedule.Stop()
		for {
			select {
			case <-ctx.Done():
				resolve(opts.failedHandler)
				return
			case <-schedule.C:
				schedule.Reset(opts.retryAfter)
				next, err := create()
				if err == nil {
					resolve(next)
					return
				}
				opts.notify(err)
			}
		}
	}()

	return &h
}
