# go-deferred

[![Build Status](https://travis-ci.org/m90/go-deferred.svg?branch=master)](https://travis-ci.org/m90/go-deferred)
[![godoc](https://godoc.org/github.com/m90/go-deferred?status.svg)](http://godoc.org/github.com/m90/go-deferred)

> defer handler creation like what?!?!?

package deferred asynchronously creates an HTTP handler by calling a function that may fail and retries in case of failure.

## Motivation

go 1.10 has an issue where you [cannot register handlers on a `ServeMux` from a goroutine](https://github.com/golang/go/pull/23994). This poses problems when creating handlers that depend on 3rd parties when those are having issues or when running your application in environments like Heroku that expect you to meet a certain boot deadline.

This package is just a workaround and should not be used when having the option to use go1.11 and above that fix the issue mentioned above.

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
	deferred.WithRetryAfter(time.Second), // retry after a second
	deferred.WithTimeoutAfter(time.Second * 15), // buffered requests will timeout after 15 seconds
	deferred.WithFailedHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "sorry, but i stopped trying", http.StatusServiceUnavailable)
	})),
)

http.Handle("/", h)
```

### License
MIT © [Frederik Ring](http://www.frederikring.com)
