package strategy

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/platinummonkey/go-concurrency-limits/core"
	"github.com/platinummonkey/go-concurrency-limits/strategy/matchers"
)

func makeTestLookupPartitions() map[string]*LookupPartition {
	batchPartition := NewLookupPartitionWithMetricRegistry(
		"batch",
		0.3,
		1,
		core.EmptyMetricRegistryInstance,
	)

	livePartition := NewLookupPartitionWithMetricRegistry(
		"live",
		0.7,
		1,
		core.EmptyMetricRegistryInstance,
	)

	return map[string]*LookupPartition{"batch": batchPartition, "live": livePartition}
}

func TestLookupPartitionStrategy(t *testing.T) {
	t.Parallel()

	t.Run("NewLookupPartitionStrategy", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		strategy, err := NewLookupPartitionStrategyWithMetricRegistry(
			makeTestLookupPartitions(),
			nil,
			10,
			core.EmptyMetricRegistryInstance,
		)
		asrt.NoError(err)
		asrt.NotNil(strategy)
		asrt.Equal(0, strategy.BusyCount())
		asrt.Equal(10, strategy.Limit())
		// check partition limits
		lmt, err := strategy.BinLimit("batch")
		asrt.NoError(err)
		asrt.Equal(3, lmt)
		cnt, err := strategy.BinBusyCount("batch")
		asrt.NoError(err)
		asrt.Equal(0, cnt)
		lmt, err = strategy.BinLimit("live")
		asrt.NoError(err)
		asrt.Equal(7, lmt)
		cnt, err = strategy.BinBusyCount("live")
		asrt.NoError(err)
		asrt.Equal(0, cnt)

		// Check stringer
		asrt.True(strings.Contains(strategy.String(), "LookupPartitionStrategy{partitions=map["))
		asrt.Equal("batch", strategy.partitions["batch"].Name())
		asrt.Equal("live", strategy.partitions["live"].Name())
	})

	t.Run("LimitAllocatedToBins", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		strategy, err := NewLookupPartitionStrategyWithMetricRegistry(
			makeTestLookupPartitions(),
			nil,
			-1, // let default 1 take over
			core.EmptyMetricRegistryInstance,
		)
		asrt.NoError(err, "failed to create strategy")
		asrt.NotNil(strategy)

		// negative limit uses 1
		strategy.SetLimit(-10)
		asrt.Equal(0, strategy.BusyCount())
		asrt.Equal(1, strategy.Limit(), "expected minLimit of 1 for invalid values.")
		cnt, err := strategy.BinBusyCount("batch")
		asrt.NoError(err)
		asrt.Equal(0, cnt)
		lmt, err := strategy.BinLimit("batch")
		asrt.NoError(err)
		asrt.Equal(1, lmt)
		cnt, err = strategy.BinBusyCount("live")
		asrt.NoError(err)
		asrt.Equal(0, cnt)
		lmt, err = strategy.BinLimit("live")
		asrt.NoError(err)
		asrt.Equal(1, lmt)
	})

	t.Run("UseExcessCapacityUntilTotalLimit", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		strategy, err := NewLookupPartitionStrategyWithMetricRegistry(
			makeTestLookupPartitions(),
			nil,
			1,
			core.EmptyMetricRegistryInstance,
		)
		asrt.NoError(err, "failed to create strategy")
		asrt.NotNil(strategy)

		strategy.SetLimit(10)

		ctx := context.WithValue(context.Background(), matchers.LookupPartitionContextKey, "batch")

		for i := 0; i < 10; i++ {
			token, ok := strategy.TryAcquire(ctx)
			asrt.True(ok && token != nil)
			asrt.True(token.IsAcquired())
			busyCount, err := strategy.BinBusyCount("batch")
			asrt.NoError(err)
			asrt.Equal(i+1, busyCount)
		}

		// should be exceeded
		token, ok := strategy.TryAcquire(ctx)
		asrt.False(ok)
		if token != nil {
			asrt.False(token.IsAcquired())
		}
	})

	t.Run("ExceedTotalLimitForUnusedBin", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		strategy, err := NewLookupPartitionStrategyWithMetricRegistry(
			makeTestLookupPartitions(),
			nil,
			1,
			core.EmptyMetricRegistryInstance,
		)
		asrt.NoError(err, "failed to create strategy")
		asrt.NotNil(strategy)

		strategy.SetLimit(10)

		ctxBatch := context.WithValue(context.Background(), matchers.LookupPartitionContextKey, "batch")
		ctxLive := context.WithValue(context.Background(), matchers.LookupPartitionContextKey, "live")

		for i := 0; i < 10; i++ {
			token, ok := strategy.TryAcquire(ctxBatch)
			asrt.True(ok && token != nil)
			asrt.True(token.IsAcquired())
			busyCount, err := strategy.BinBusyCount("batch")
			asrt.NoError(err)
			asrt.Equal(i+1, busyCount)
		}

		// should be exceeded
		token, ok := strategy.TryAcquire(ctxBatch)
		asrt.False(ok)
		if token != nil {
			asrt.False(token.IsAcquired())
		}

		// now try live
		token, ok = strategy.TryAcquire(ctxLive)
		asrt.True(ok && token != nil)
		asrt.True(token.IsAcquired())
	})

	t.Run("RejectOnceAllLimitsReached", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		strategy, err := NewLookupPartitionStrategyWithMetricRegistry(
			makeTestLookupPartitions(),
			nil,
			1,
			core.EmptyMetricRegistryInstance,
		)
		asrt.NoError(err, "failed to create strategy")
		asrt.NotNil(strategy)

		strategy.SetLimit(10)

		ctxBatch := context.WithValue(context.Background(), matchers.LookupPartitionContextKey, "batch")
		ctxLive := context.WithValue(context.Background(), matchers.LookupPartitionContextKey, "live")

		for i := 0; i < 3; i++ {
			token, ok := strategy.TryAcquire(ctxBatch)
			asrt.True(ok && token != nil)
			asrt.True(token.IsAcquired())
			busyCount, err := strategy.BinBusyCount("batch")
			asrt.NoError(err)
			asrt.Equal(i+1, busyCount)
			asrt.Equal(i+1, strategy.BusyCount())
		}

		for i := 0; i < 7; i++ {
			token, ok := strategy.TryAcquire(ctxLive)
			asrt.True(ok && token != nil)
			asrt.True(token.IsAcquired())
			busyCount, err := strategy.BinBusyCount("live")
			asrt.NoError(err)
			asrt.Equal(i+1, busyCount)
			asrt.Equal(i+4, strategy.BusyCount())
		}

		// should be exceeded
		token, ok := strategy.TryAcquire(ctxBatch)
		asrt.False(ok)
		if token != nil {
			asrt.False(token.IsAcquired())
		}
		// should be exceeded
		token, ok = strategy.TryAcquire(ctxLive)
		asrt.False(ok)
		if token != nil {
			asrt.False(token.IsAcquired())
		}

	})

	t.Run("ReleaseLimit", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		strategy, err := NewLookupPartitionStrategyWithMetricRegistry(
			makeTestLookupPartitions(),
			nil,
			1,
			core.EmptyMetricRegistryInstance,
		)
		asrt.NoError(err, "failed to create strategy")
		asrt.NotNil(strategy)

		strategy.SetLimit(10)

		ctxBatch := context.WithValue(context.Background(), matchers.LookupPartitionContextKey, "batch")

		token, ok := strategy.TryAcquire(ctxBatch)
		asrt.True(ok && token != nil)
		asrt.True(token.IsAcquired())

		for i := 1; i < 10; i++ {
			token2, ok := strategy.TryAcquire(ctxBatch)
			asrt.True(ok && token2 != nil)
			asrt.True(token2.IsAcquired())
			busyCount, err := strategy.BinBusyCount("batch")
			asrt.NoError(err)
			asrt.Equal(i+1, busyCount)
		}

		// should be exceeded
		token2, ok := strategy.TryAcquire(ctxBatch)
		asrt.False(ok)
		if token2 != nil {
			asrt.False(token2.IsAcquired())
		}

		token.Release()
		busyCount, err := strategy.BinBusyCount("batch")
		asrt.NoError(err)
		asrt.Equal(9, busyCount)
		asrt.Equal(9, strategy.BusyCount())

		// can re-acquire
		token2, ok = strategy.TryAcquire(ctxBatch)
		asrt.True(ok && token2 != nil)
		asrt.True(token2.IsAcquired())

		busyCount, err = strategy.BinBusyCount("batch")
		asrt.NoError(err)
		asrt.Equal(10, busyCount)
		asrt.Equal(10, strategy.BusyCount())
	})

	t.Run("SetLimitReservesBusy", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		strategy, err := NewLookupPartitionStrategyWithMetricRegistry(
			makeTestLookupPartitions(),
			nil,
			1,
			core.EmptyMetricRegistryInstance,
		)
		asrt.NoError(err, "failed to create strategy")
		asrt.NotNil(strategy)

		strategy.SetLimit(10)

		binLimit, err := strategy.BinLimit("batch")
		asrt.NoError(err)
		asrt.Equal(3, binLimit)

		ctxBatch := context.WithValue(context.Background(), matchers.LookupPartitionContextKey, "batch")
		// should be exceeded
		token, ok := strategy.TryAcquire(ctxBatch)
		asrt.True(ok && token != nil)
		asrt.True(token.IsAcquired())

		busyCount, err := strategy.BinBusyCount("batch")
		asrt.NoError(err)
		asrt.Equal(1, busyCount)
		asrt.Equal(1, strategy.BusyCount())

		strategy.SetLimit(20)

		binLimit, err = strategy.BinLimit("batch")
		asrt.NoError(err)
		asrt.Equal(6, binLimit)

		busyCount, err = strategy.BinBusyCount("batch")
		asrt.NoError(err)
		asrt.Equal(1, busyCount)
		asrt.Equal(1, strategy.BusyCount())
	})

	t.Run("AddRemoveDynamically", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		testPartitions := makeTestLookupPartitions()
		strategy, err := NewLookupPartitionStrategyWithMetricRegistry(
			testPartitions,
			nil,
			1,
			core.EmptyMetricRegistryInstance,
		)
		asrt.NoError(err, "failed to create strategy")
		asrt.NotNil(strategy)

		// add a partition
		testPartition := NewLookupPartitionWithMetricRegistry(
			"test1",
			0.7,
			1,
			core.EmptyMetricRegistryInstance,
		)
		strategy.AddPartition(testPartition.Name(), testPartition)
		binLimit, err := strategy.BinLimit("test1")
		asrt.NoError(err)
		asrt.Equal(1, binLimit)

		// remove a partition
		strategy.RemovePartition("test1")
		binLimit, err = strategy.BinLimit("test1")
		asrt.Error(err)
	})
}
