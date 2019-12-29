package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit"
	"github.com/platinummonkey/go-concurrency-limits/limiter"
	"github.com/platinummonkey/go-concurrency-limits/strategy"
)

type contextKey string

const testContextKey contextKey = "jobID"

type resource struct {
	counter *int64
}

func (r *resource) poll(ctx context.Context) (bool, error) {
	currentCount := atomic.AddInt64(r.counter, 1)
	id := ctx.Value(testContextKey).(int)
	log.Printf("request started for id=%d currentCount=%d\n", id, currentCount)
	numKeys := rand.Int63n(1000)

	// fake a scan, every key lookup additional non-linear time
	scanTime := time.Duration(numKeys)*time.Nanosecond + time.Duration(int64(math.Exp(float64(numKeys)/100.0)))*time.Millisecond
	scanTimeMillis := scanTime.Milliseconds()

	// sleep some time
	time.Sleep(scanTime)
	currentCount = atomic.AddInt64(r.counter, -1)
	log.Printf("request succeeded for id=%d currentCount=%d scanTime=%d ms\n", id, currentCount, scanTimeMillis)
	return true, nil
}

type protectedResource struct {
	external *resource
	guard    core.Limiter
}

func (r *protectedResource) poll(ctx context.Context) (bool, error) {
	id := ctx.Value(testContextKey).(int)
	log.Printf("guarded request started for id=%d\n", id)
	token, ok := r.guard.Acquire(ctx)
	if !ok {
		// short circuit no need to try
		log.Printf("guarded request short circuited for id=%d\n", id)
		if token != nil {
			token.OnDropped()
		}
		return false, fmt.Errorf("short circuited request id=%d", id)
	}

	// try to make request
	_, err := r.external.poll(ctx)
	if err != nil {
		token.OnDropped()
		log.Printf("guarded request failed for id=%d err=%v\n", id, err)
		return false, fmt.Errorf("request failed err=%v", err)
	}
	token.OnSuccess()
	log.Printf("guarded request succeeded for id=%d\n", id)
	return true, nil
}

func main() {
	l := 1000
	limitStrategy := strategy.NewSimpleStrategy(l)
	logger := limit.BuiltinLimitLogger{}
	defaultLimiter, err := limiter.NewDefaultLimiter(
		limit.NewFixedLimit(
			"initializer_limiter",
			l,
			core.EmptyMetricRegistryInstance,
		),
		int64(time.Millisecond*250),
		int64(time.Millisecond*500),
		int64(time.Millisecond*10),
		100,
		limitStrategy,
		logger,
		core.EmptyMetricRegistryInstance,
	)
	externalResourceLimiter := limiter.NewBlockingLimiter(defaultLimiter, time.Second, logger)

	if err != nil {
		log.Fatalf("Error creating limiter err=%v\n", err)
		os.Exit(-1)
	}

	initialCount := int64(0)
	fakeExternalResource := &resource{
		counter: &initialCount,
	}

	guardedResource := &protectedResource{
		external: fakeExternalResource,
		guard:    externalResourceLimiter,
	}
	atomic.StoreInt64(fakeExternalResource.counter, 0)

	endOfExampleTimer := time.NewTimer(time.Second * 10)
	wg := sync.WaitGroup{}

	// spin up 10*l consumers
	wg.Add(10 * l)
	for i := 0; i < 10*l; i++ {
		go func(c int) {
			for i := 0; i < 5; i++ {
				defer wg.Done()
				ctx := context.WithValue(context.Background(), testContextKey, c+i)
				guardedResource.poll(ctx)
			}
		}(i)
	}

	<-endOfExampleTimer.C
	log.Printf("Waiting for go-routines to finish...")
	wg.Wait()
}
