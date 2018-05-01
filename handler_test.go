package deferred

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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
		h.ServeHTTP(w1, r1)

		if w1.Code != http.StatusOK {
			t.Errorf("Unexpected status code %v", w1.Code)
		}

		if w1.Body.String() != "hello" {
			t.Errorf("Unexpected body %v", w1.Body.String())
		}

		w2, r2 := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/?query=hello", nil)
		h.ServeHTTP(w2, r2)

		if w2.Code != http.StatusOK {
			t.Errorf("Unexpected status code %v", w2.Code)
		}

		if w2.Body.String() != "hello" {
			t.Errorf("Unexpected body %v", w2.Body.String())
		}
	})

	t.Run("context", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var passedError error
		h := NewHandler(
			ctx,
			func() (http.Handler, error) {
				return nil, errors.New("not yet")
			},
			WithNotify(func(err error) {
				passedError = err
			}),
			WithRetryAfter(time.Minute),
			WithTimeoutAfter(time.Minute),
		)

		w1, r1 := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil)
		h.ServeHTTP(w1, r1)

		if w1.Code != http.StatusServiceUnavailable {
			t.Errorf("Unexpected status code %v", w1.Code)
		}

		w2, r2 := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil)
		h.ServeHTTP(w2, r2)

		if w2.Code != http.StatusServiceUnavailable {
			t.Errorf("Unexpected status code %v", w2.Code)
		}

		if passedError == nil {
			t.Error("notify not called with error")
		}
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
			WithRetryAfter(time.Second),
		)

		for i := 0; i < 5; i++ {
			w, r := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil)
			h.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("Unexpected status code %v", w.Code)
			}
			time.Sleep(time.Second)
		}

		w, r := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil)
		h.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("Unexpected status code %v", w.Code)
		}

		if len(errors) != 5 {
			t.Errorf("Unexpected list of errors %v", errors)
		}
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
