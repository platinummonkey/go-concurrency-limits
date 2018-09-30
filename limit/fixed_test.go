package limit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFixedLimit(t *testing.T) {
	t.Parallel()
	asrt := assert.New(t)
	l := NewFixedLimit("test", 10, nil)
	asrt.Equal(10, l.EstimatedLimit())

	l.OnSample(0, (time.Millisecond * 10).Nanoseconds(), 10, false)
	asrt.Equal(10, l.EstimatedLimit())

	l.OnSample(0, (time.Millisecond * 10).Nanoseconds(), 100, false)
	asrt.Equal(10, l.EstimatedLimit())

	l.OnSample(0, (time.Millisecond * 10).Nanoseconds(), 100, true)
	asrt.Equal(10, l.EstimatedLimit())

	// NOOP
	listener := testNotifyListener{}
	l.NotifyOnChange(listener.updater())

	asrt.Equal("FixedLimit{limit=10}", l.String())
}
