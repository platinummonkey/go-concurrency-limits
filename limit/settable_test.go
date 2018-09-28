package limit

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/platinummonkey/go-concurrency-limits/core"
)

type testNotifyListener struct {
	changes []int
	mu sync.Mutex
}

func (l *testNotifyListener) updater() core.LimitChangeListener {
	return func(limit int) {
		l.mu.Lock()
		l.changes = append(l.changes, limit)
		l.mu.Unlock()
	}
}

func TestSettableLimit(t *testing.T) {
	t.Parallel()
	asrt := assert.New(t)
	l := NewSettableLimit(10)
	asrt.Equal(10, l.EstimatedLimit())

	l.SetLimit(5)
	asrt.Equal(5, l.EstimatedLimit())

	// should be a noop
	l.OnSample(0, 10, 1, true)
	asrt.Equal(5, l.EstimatedLimit())

	asrt.Equal("SettableLimit{limit=5}", l.String())

	// notify on change
	listener := testNotifyListener{changes: make([]int, 0)}
	l.NotifyOnChange(listener.updater())
	l.SetLimit(10)
	l.SetLimit(5)
	l.SetLimit(1)
	asrt.Equal(10, listener.changes[0])
	asrt.Equal(5, listener.changes[1])
	asrt.Equal(1, listener.changes[2])
}
