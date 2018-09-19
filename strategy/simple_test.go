package strategy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimpleStrategy(t *testing.T) {
	t.Run("LimitLessThanOneSetAsOne", func(t2 *testing.T) {
		asrt := assert.New(t2)
		strategy := NewSimpleStrategy(-10)
		asrt.Equal(1, strategy.GetLimit(), "expected a default limit of 1")
	})

	t.Run("InitialState", func(t2 *testing.T) {
		asrt := assert.New(t2)
		strategy := NewSimpleStrategy(1)
		asrt.Equal(1, strategy.GetLimit(), "expected a default limit of 1")
		asrt.Equal(0, strategy.GetBusyCount(), "expected all resources free")
		asrt.Contains(strategy.String(), "SimpleStrategy{inFlight=0, ")
	})

	t.Run("SetLimit", func(t2 *testing.T) {
		asrt := assert.New(t2)
		strategy := NewSimpleStrategy(0)
		asrt.Equal(1, strategy.GetLimit(), "expected a default limit of 3")
		strategy.SetLimit(2)
		asrt.Equal(2, strategy.GetLimit())
		// negative limits result in 1
		strategy.SetLimit(-10)
		asrt.Equal(1, strategy.GetLimit())
		asrt.Equal(0, strategy.GetBusyCount(), "expected all resources free")
	})

	t.Run("AcquireIncrementsBusy", func(t2 *testing.T) {
		asrt := assert.New(t2)
		strategy := NewSimpleStrategy(1)
		asrt.Equal(0, strategy.GetBusyCount(), "expected all resources free")
		token, ok := strategy.TryAcquire(context.Background())
		asrt.True(ok && token != nil, "expected token")
		asrt.True(token.IsAcquired(), "expected acquired token")
		asrt.Equal(1, strategy.GetBusyCount(), "expected 1 resource taken")
	})

	t.Run("ExceedingLimitReturnsFalse", func(t2 *testing.T) {
		asrt := assert.New(t2)
		strategy := NewSimpleStrategy(1)
		asrt.Equal(0, strategy.GetBusyCount(), "expected all resources free")
		token, ok := strategy.TryAcquire(context.Background())
		asrt.True(ok && token != nil, "expected token")
		asrt.True(token.IsAcquired(), "expected acquired token")
		asrt.Equal(1, strategy.GetBusyCount(), "expected 1 resource taken")

		// try again but we expect this to fail
		token2, ok2 := strategy.TryAcquire(context.Background())
		asrt.False(ok2, "expected token fail")
		if token2 != nil {
			asrt.False(token2.IsAcquired(), "token should not be acquired")
		}
		asrt.Equal(1, strategy.GetBusyCount(), "expected only 1 resource taken")
	})

	t.Run("AcquireAndRelease", func(t2 *testing.T) {
		asrt := assert.New(t2)
		strategy := NewSimpleStrategy(1)
		asrt.Equal(0, strategy.GetBusyCount(), "expected all resources free")
		token, ok := strategy.TryAcquire(context.Background())
		asrt.True(ok && token != nil, "expected token")
		asrt.True(token.IsAcquired(), "expected acquired token")
		asrt.Equal(1, strategy.GetBusyCount(), "expected 1 resource taken")

		token.Release()

		asrt.Equal(0, strategy.GetBusyCount(), "expected all resources free")

		token, ok = strategy.TryAcquire(context.Background())
		asrt.True(ok && token != nil, "expected token")
		asrt.True(token.IsAcquired(), "expected acquired token")
		asrt.Equal(1, strategy.GetBusyCount(), "expected 1 resource taken")
	})
}
