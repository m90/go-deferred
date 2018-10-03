package deferred

import (
	"math"
	"math/rand"
	"time"
)

const (
	maxDelay   = 1000 * time.Second
	factor     = 1.6
	maxFloat64 = float64(math.MaxInt64 - 512)
)

type strategy interface {
	backoff() time.Duration
}

type retry struct {
	delay time.Duration
}

func (r retry) backoff() time.Duration {
	return r.delay
}

type exponential struct {
	initDelay time.Duration
	retryNum  float64
}

func (ex *exponential) backoff() time.Duration {
	min := ex.initDelay
	if min <= 0 {
		min = 100 * time.Millisecond
	}

	if min >= maxDelay {
		return maxDelay
	}

	//calculate this duration
	minf := float64(min)
	durf := minf * math.Pow(factor, ex.retryNum)
	ex.retryNum++
	durf = rand.Float64()*(durf-minf) + minf
	//ensure float64 wont overflow int64
	if durf > maxFloat64 {
		return maxDelay
	}
	dur := time.Duration(durf)
	//keep within bounds
	if dur < min {
		return min
	}
	if dur > maxDelay {
		return maxDelay
	}
	return dur
}
