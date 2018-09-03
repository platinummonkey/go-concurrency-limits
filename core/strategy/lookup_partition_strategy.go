package strategy

import (
	"context"

	"github.com/platinummonkey/go-concurrency-limits/core"
)

// PARTITION_TAG_NAME represents the metric tag used for the partition identifier
const PARTITION_TAG_NAME = "partition"

type LookupPartitionStrategy struct {

}

func (*LookupPartitionStrategy) TryAcquire(ctx context.Context) (token core.StrategyToken, ok bool) {
	panic("implement me")
}

func (*LookupPartitionStrategy) SetLimit(limit int) {
	panic("implement me")
}

