package core

import (
	"context"
)

type StrategyToken interface {
	// IsAcquired returns true if acquired or false if limit has been reached.
	IsAcquired() bool
	// InFlightCount will return the number of pending requests.
	InFlightCount() int
	// Release the acquired token and decrement the current in-flight count.
	Release()
}

type StaticStrategyToken struct {
	acquired bool
	inFlightCount int
	releaseFunc func()
}

func (t *StaticStrategyToken) IsAcquired() bool {
	return t.acquired
}

func (t *StaticStrategyToken) InFlightCount() int {
	return t.inFlightCount
}

func (t *StaticStrategyToken) Release() {
	if t.releaseFunc != nil {
		t.releaseFunc()
	}
}

func NewNotAcquiredStrategyToken(inFlightCount int) StrategyToken {
	return &StaticStrategyToken{
		acquired: false,
		inFlightCount: inFlightCount,
		releaseFunc: func() {},
	}
}

func NewAcquiredStrategyToken(inFlightCount int, releaseFunc func()) StrategyToken {
	return &StaticStrategyToken{
		acquired: true,
		inFlightCount: inFlightCount,
		releaseFunc: releaseFunc,
	}
}

type Strategy interface {
	// TryAcquire will try to acquire a token from the limiter.
	// context Context of the request for partitioned limits.
	// returns not ok if limit is exceeded, or a StrategyToken that must be released when the operation completes.
	TryAcquire(ctx context.Context) (token StrategyToken, ok bool)

	// SetLimit will update the strategy with a new limit.
	SetLimit(limit int)
}
