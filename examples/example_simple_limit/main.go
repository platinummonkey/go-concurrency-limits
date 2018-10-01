package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"golang.org/x/time/rate"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit"
	"github.com/platinummonkey/go-concurrency-limits/limiter"
	"github.com/platinummonkey/go-concurrency-limits/strategy"
)

type contextKey uint8

const testContextKey = contextKey(1)

type resource struct {
	limiter *rate.Limiter
}

func (r *resource) poll(ctx context.Context) (bool, error) {
	id := ctx.Value(testContextKey).(int)
	log.Printf("request started for id=%d\n", id)
	if !r.limiter.Allow() {
		time.Sleep(time.Millisecond * 10)
		return false, fmt.Errorf("limit exceeded for id=%d", id)
	}
	// sleep some time
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(90)+10))
	log.Printf("request succeeded for id=%d\n", id)
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
	limitStrategy := strategy.NewSimpleStrategy(10)
	externalResourceLimiter, err := limiter.NewDefaultLimiterWithDefaults(
		"example_single_limit",
		limitStrategy,
		limit.BuiltinLimitLogger{},
		core.EmptyMetricRegistryInstance,
	)
	if err != nil {
		log.Fatalf("Error creating limiter err=%v\n", err)
		os.Exit(-1)
	}

	fakeExternalResource := &resource{
		limiter: rate.NewLimiter(5, 15),
	}

	guardedResource := &protectedResource{
		external: fakeExternalResource,
		guard:    externalResourceLimiter,
	}

	endOfExampleTimer := time.NewTimer(time.Second * 60)
	ticker := time.NewTicker(time.Millisecond * 100)
	counter := 0
	for {
		select {
		case <-endOfExampleTimer.C:
			return
		case <-ticker.C:
			// make a request
			go func() {
				ctx := context.WithValue(context.Background(), testContextKey, counter)
				guardedResource.poll(ctx)
			}()
		}
		counter++
	}
}
