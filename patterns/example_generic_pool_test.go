package patterns

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/platinummonkey/go-concurrency-limits/limit"
	"github.com/platinummonkey/go-concurrency-limits/limiter"
	"github.com/platinummonkey/go-concurrency-limits/strategy"
)

func ExampleGenericPool() {
	var JobKey = "job_id"

	l := 1000 // limit to adjustable 1000 concurrent requests.
	delegateLimit := limit.NewDefaultAIMLimit(
		"aimd_limiter",
		nil,
	)
	// wrap with a default limiter and simple strategy
	// you could of course get very complicated with this.
	delegateLimiter, err := limiter.NewDefaultLimiter(
		delegateLimit,
		(time.Millisecond*250).Nanoseconds(),
		(time.Millisecond*500).Nanoseconds(),
		(time.Millisecond*10).Nanoseconds(),
		100,
		strategy.NewSimpleStrategy(l),
		limit.BuiltinLimitLogger{},
		nil,
	)
	if err != nil {
		panic(err)
	}

	// create a new pool
	pool, err := NewPool(
		delegateLimiter,
		false,
		0,
		time.Second,
		limit.BuiltinLimitLogger{},
		nil,
	)
	if err != nil {
		panic(err)
	}

	wg := sync.WaitGroup{}
	wg.Add(l * 3)
	// spawn 3000 concurrent requests that would normally be too much load for the protected resource.
	for i := 0; i <= l * 3; i++ {
		go func(c int) {
			defer wg.Done()
			ctx := context.WithValue(context.Background(), JobKey, c)
			// this will block until timeout or token was acquired.
			listener, ok := pool.Acquire(ctx)
			if !ok {
				log.Printf("was not able to acquire lock for id=%d\n", c)
				return
			}
			log.Printf("acquired lock for id=%d\n", c)
			// do something...
			time.Sleep(time.Millisecond*10)
			listener.OnSuccess()
			log.Printf("released lock for id=%d\n", c)
		}(i)
	}

	// wait for completion
	wg.Wait()
	log.Println("Finished")
}
