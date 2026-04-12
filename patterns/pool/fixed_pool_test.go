package pool

import (
	"context"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testKey string

const testKeyID = testKey("id")

func TestFixedPool(t *testing.T) {
	asrt := assert.New(t)
	p, err := NewFixedPool(
		"test-fixed-pool",
		OrderingRandom,
		10,
		-1,
		-1,
		-1,
		-1,
		0,
		0,
		nil,
		nil,
	)
	asrt.NoError(err)

	asrt.Equal(10, p.Limit())

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(c int) {
			defer wg.Done()
			// OrderingRandom uses BlockingLimiter with timeout=0, which blocks
			// indefinitely on a non-cancelled context until a slot is available.
			// l is guaranteed non-nil here because context.Background() is never cancelled.
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

func TestFIFOFixedPool(t *testing.T) {
	asrt := assert.New(t)
	p, err := NewFixedPool(
		"test-fifo-fixed-pool",
		OrderingFIFO,
		10,
		-1,
		-1,
		-1,
		-1,
		0,
		0,
		nil,
		nil,
	)
	asrt.NoError(err)

	asrt.Equal(10, p.Limit())

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(c int) {
			defer wg.Done()
			// maxBacklog=0 means no queuing: Acquire may return nil if the limit is
			// already saturated and no backlog slot is available.
			l, ok := p.Acquire(context.WithValue(context.Background(), testKeyID, fmt.Sprint(c)))
			if !ok {
				log.Printf("not acquired (backlog full) - %d\n", c)
				return
			}
			log.Printf("acquired now, sleeping - %d\n", c)
			time.Sleep(time.Millisecond * 100)
			l.OnSuccess()
			log.Printf("no longer acquired, released - %d\n", c)
		}(i)
		time.Sleep(time.Millisecond * 5)
	}
	wg.Wait()
}

func TestLIFOFixedPool(t *testing.T) {
	asrt := assert.New(t)
	p, err := NewFixedPool(
		"test-lifo-fixed-pool",
		OrderingLIFO,
		10,
		-1,
		-1,
		-1,
		-1,
		0,
		0,
		nil,
		nil,
	)
	asrt.NoError(err)

	asrt.Equal(10, p.Limit())

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(c int) {
			defer wg.Done()
			// maxBacklog=0 means no queuing: Acquire may return nil if the limit is
			// already saturated and no backlog slot is available.
			l, ok := p.Acquire(context.WithValue(context.Background(), testKeyID, fmt.Sprint(c)))
			if !ok {
				log.Printf("not acquired (backlog full) - %d\n", c)
				return
			}
			log.Printf("acquired now, sleeping - %d\n", c)
			time.Sleep(time.Millisecond * 100)
			l.OnSuccess()
			log.Printf("no longer acquired, released - %d\n", c)
		}(i)
		time.Sleep(time.Millisecond * 5)
	}
	wg.Wait()
}
