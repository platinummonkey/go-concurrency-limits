package limiter

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/limit"
	"github.com/platinummonkey/go-concurrency-limits/strategy"
	"github.com/stretchr/testify/assert"
)

func TestDeadlineLimiter(t *testing.T) {
	t.Run("Unblocked", func(t2 *testing.T) {
		asrt := assert.New(t2)
		l := limit.NewSettableLimit("test", 10, nil)
		noopLogger := limit.NoopLimitLogger{}
		defaultLimiter, err := NewDefaultLimiter(
			l,
			defaultMinWindowTime,
			defaultMaxWindowTime,
			defaultMinRTTThreshold,
			defaultWindowSize,
			strategy.NewSimpleStrategy(10),
			noopLogger,
			core.EmptyMetricRegistryInstance,
		)
		if !asrt.NoError(err) {
			asrt.FailNow("")
		}
		asrt.NotNil(defaultLimiter)
		deadline := time.Now().Add(time.Second*15)
		deadlineLimiter := NewDeadlineLimiter(defaultLimiter, deadline, noopLogger)
		// stringer
		asrt.True(strings.Contains(deadlineLimiter.String(), "DeadlineLimiter{delegate=DefaultLimiter{"))

		var listeners []core.Listener
		for i := 0; i < 10; i++ {
			listener, ok := deadlineLimiter.Acquire(context.Background())
			if ok && listener != nil {
				listeners = append(listeners, listener)
			}
		}

		l.SetLimit(1)

		for _, listener := range listeners {
			listener.OnSuccess()
		}

		deadlineLimiter.Acquire(context.Background())
	})

	t.Run("Deadline passed", func(t2 *testing.T) {
		asrt := assert.New(t2)
		l := limit.NewSettableLimit("test", 1, nil)
		noopLogger := limit.NoopLimitLogger{}
		defaultLimiter, err := NewDefaultLimiter(
			l,
			defaultMinWindowTime,
			defaultMaxWindowTime,
			defaultMinRTTThreshold,
			defaultWindowSize,
			strategy.NewSimpleStrategy(1),
			noopLogger,
			core.EmptyMetricRegistryInstance,
		)
		if !asrt.NoError(err) {
			asrt.FailNow("")
		}
		asrt.NotNil(defaultLimiter)
		deadline := time.Now().Add(time.Second*1)
		deadlineLimiter := NewDeadlineLimiter(defaultLimiter, deadline, noopLogger)

		i := 1
		deadlineFound := false

		for {
			listener, ok := deadlineLimiter.Acquire(context.Background())
			if ok && listener != nil {
				listener.OnSuccess()
				time.Sleep(time.Second)
			} else if i > 3 {
				break
			} else {
				deadlineFound = true
				break
			}
			i++
		}

		asrt.True(deadlineFound, "expected deadline to be reached but not after %d attempts", i)
		asrt.Equal(2, i, "expected deadline to be exceeded on second attempt")
	})
}

