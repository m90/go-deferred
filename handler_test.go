package deferred

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		h := NewHandler(
			context.Background(),
			func() (http.Handler, error) {
				time.Sleep(time.Second)
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte(r.URL.Query().Get("query")))
				}), nil
			},
			WithNotify(func(err error) {
				t.Fatalf("Unexpected error %v", err)
			}),
		)

		w1, r1 := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/?query=hello", nil)
		w2, r2 := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/?query=hello", nil)

		wg := sync.WaitGroup{}
		wg.Add(2)

		go func() {
			h.ServeHTTP(w1, r1)
			if w1.Code != http.StatusOK {
				t.Errorf("Unexpected status code %v", w1.Code)
			}
			if w1.Body.String() != "hello" {
				t.Errorf("Unexpected body %v", w1.Body.String())
			}
			wg.Done()
		}()

		go func() {
			time.Sleep(time.Second * 2)
			h.ServeHTTP(w2, r2)
			if w2.Code != http.StatusOK {
				t.Errorf("Unexpected status code %v", w2.Code)
			}
			if w2.Body.String() != "hello" {
				t.Errorf("Unexpected body %v", w2.Body.String())
			}
			wg.Done()
		}()

		wg.Wait()
	})

	t.Run("context", func(t *testing.T) {
		mu := sync.Mutex{}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var passedError error
		h := NewHandler(
			ctx,
			func() (http.Handler, error) {
				return nil, errors.New("not yet")
			},
			WithNotify(func(err error) {
				mu.Lock()
				passedError = err
				mu.Unlock()
			}),
			WithRetryAfter(time.Minute),
			WithTimeoutAfter(time.Minute),
			WithFailedHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "nope", http.StatusNotImplemented)
			})),
		)

		wg := sync.WaitGroup{}
		wg.Add(2)

		w1, r1 := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil)
		w2, r2 := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil)

		go func() {
			h.ServeHTTP(w1, r1)
			if w1.Code != http.StatusNotImplemented {
				t.Errorf("Unexpected status code %v", w1.Code)
			}
			wg.Done()
		}()

		go func() {
			time.Sleep(time.Second * 2)
			h.ServeHTTP(w2, r2)
			if w2.Code != http.StatusNotImplemented {
				t.Errorf("Unexpected status code %v", w2.Code)
			}
			wg.Done()
		}()

		time.Sleep(time.Second)

		mu.Lock()
		if passedError == nil {
			t.Error("notify not called with error")
		}
		mu.Unlock()

		wg.Wait()
	})

	t.Run("retries", func(t *testing.T) {
		errors := []error{}
		count := 0
		h := NewHandler(
			context.Background(),
			func() (http.Handler, error) {
				if count < 5 {
					count++
					return nil, fmt.Errorf("count %v still lt 5", count)
				}
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Write([]byte("finally"))
				}), nil
			},
			WithNotify(func(err error) {
				errors = append(errors, err)
			}),
			WithRetryAfter(time.Second/10),
		)

		wg := sync.WaitGroup{}
		wg.Add(5)

		for i := 0; i < 5; i++ {
			go func() {
				w, r := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil)
				h.ServeHTTP(w, r)
				if w.Code != http.StatusOK {
					t.Errorf("Unexpected status code %v", w.Code)
				}
				wg.Done()
			}()
		}

		time.Sleep(time.Second)

		w, r := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil)
		h.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("Unexpected status code %v", w.Code)
		}

		if len(errors) != 5 {
			t.Errorf("Unexpected list of errors %v", errors)
		}

		wg.Wait()
	})

	t.Run("timeout", func(t *testing.T) {
		h := NewHandler(
			context.Background(),
			func() (http.Handler, error) {
				return nil, errors.New("not yet")
			},
			WithTimeoutAfter(time.Second),
		)

		w, r := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil)
		h.ServeHTTP(w, r)

		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("Unexpected status code %v", w.Code)
		}
	})
}
