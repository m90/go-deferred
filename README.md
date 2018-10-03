# go-deferred

[![Build Status](https://travis-ci.org/m90/go-deferred.svg?branch=master)](https://travis-ci.org/m90/go-deferred)
[![godoc](https://godoc.org/github.com/m90/go-deferred?status.svg)](http://godoc.org/github.com/m90/go-deferred)

> defer handler creation like what?!?!?

package deferred asynchronously creates an HTTP handler by calling a function that may fail and retries in case of failure.

## Example

```go
// only return a http.Handler in some cases
func createLuckyHandler() (http.Handler, error) {
	num := rand.Intn(10)
	if num != 7 {
		return nil, fmt.Errorf("%d is not the lucky number", num)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Lucky you!"))
	})
}

timeout, cancel := context.WithTimeout(context.Background(), time.Hour)
defer cancel()

// NewHandler returns instantly
h := deferred.NewHandler(
	// in case the context is cancelled the handler will stop buffering and fail permanently
	timeout,
	// in case no error is returned, the handler will stop buffering and use the returned handler
	createLuckyHandler,
	// optional settings
	deferred.WithNotify(err error) { // subscribe to errors
		fmt.Println("got error: ", err, ", trying again")
	},
	deferred.WithExponentialBackoff(time.Second), // retry after a second, then use exponential backoff
	deferred.WithTimeoutAfter(time.Second * 30), // buffered requests will timeout after 15 seconds
	deferred.WithFailedHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "sorry, but i stopped trying", http.StatusServiceUnavailable)
	})),
)

http.Handle("/", h)
```

### License
MIT © [Frederik Ring](http://www.frederikring.com)
