package pool

import (
	"context"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/platinummonkey/go-concurrency-limits/limit"
	"github.com/platinummonkey/go-concurrency-limits/limiter"
	"github.com/platinummonkey/go-concurrency-limits/strategy"
	"github.com/stretchr/testify/assert"
)

func TestGenericPool(t *testing.T) {
	asrt := assert.New(t)
	delegateLimiter, err := limiter.NewDefaultLimiter(
		limit.NewFixedLimit("test-generic-pool", 10, nil),
		(time.Millisecond * 250).Nanoseconds(),
		(time.Millisecond * 500).Nanoseconds(),
		(time.Millisecond * 10).Nanoseconds(),
		100,
		strategy.NewPreciseStrategy(10),
		nil,
		nil,
	)
	asrt.NoError(err)

	p, err := NewPool(
		delegateLimiter,
		false,
		10,
		-1,
		nil,
		nil,
	)
	asrt.NoError(err)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(c int) {
			defer wg.Done()
			l, _ := p.Acquire(context.WithValue(context.Background(), testKeyID, fmt.Sprint(c)))
			log.Printf("acquired now, sleeping - %d\n", c)
			time.Sleep(time.Millisecond * 100)
			l.OnSuccess()
			log.Printf("no longer acquired, released - %d\n", c)
		}(i)
		time.Sleep(time.Millisecond * 5)
	}
	wg.Wait()
}
