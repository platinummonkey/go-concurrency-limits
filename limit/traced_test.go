package limit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/platinummonkey/go-concurrency-limits/measurements"
)

func TestNoopLimitLogger(t *testing.T) {
	t.Parallel()
	asrt := assert.New(t)
	l := NoopLimitLogger{}
	asrt.NotPanics(func() { l.Debugf("") })
	asrt.False(l.IsDebugEnabled())
	asrt.Equal("NoopLimitLogger{}", l.String())
}

func TestBuiltinLimitLogger(t *testing.T) {
	t.Parallel()
	asrt := assert.New(t)
	l := BuiltinLimitLogger{}
	asrt.NotPanics(func() { l.Debugf("") })
	asrt.True(l.IsDebugEnabled())
	asrt.Equal("BuiltinLimitLogger{}", l.String())
}

func TestTracedLimit(t *testing.T) {
	t.Parallel()
	asrt := assert.New(t)
	delegate := NewSettableLimit(10)
	l := NewTracedLimit(delegate, NoopLimitLogger{})

	asrt.Equal(10, l.EstimatedLimit())

	m := measurements.NewDefaultImmutableSampleWindow().AddDroppedSample(1)
	l.Update(m)
	asrt.Equal(10, l.EstimatedLimit())

	asrt.Equal("TracedLimit{limit=SettableLimit{limit=10}, logger=NoopLimitLogger{}}", l.String())
}
