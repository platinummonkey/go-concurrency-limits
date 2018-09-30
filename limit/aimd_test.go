package limit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAIMDLimit(t *testing.T) {
	t.Parallel()

	t.Run("DefaultAIMD", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := NewDefaultAIMLimit("test", nil)
		asrt.Equal(10, l.EstimatedLimit())
	})

	t.Run("Default", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := NewAIMDLimit("test", 10, 0.9, nil)
		asrt.Equal(10, l.EstimatedLimit())
		asrt.Equal(0.9, l.BackOffRatio())
	})

	t.Run("IncreaseOnSuccess", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := NewAIMDLimit("test", 10, 0.9, nil)
		listener := testNotifyListener{changes: make([]int, 0)}
		l.NotifyOnChange(listener.updater())
		l.OnSample(-1, (time.Millisecond * 1).Nanoseconds(), 10, false)
		asrt.Equal(11, l.EstimatedLimit())
		asrt.Equal(11, listener.changes[0])
	})

	t.Run("DecreaseOnDrops", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := NewAIMDLimit("test", 10, 0.9, nil)
		l.OnSample(-1, 1, 1, true)
		asrt.Equal(9, l.EstimatedLimit())
	})

	t.Run("String", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		l := NewAIMDLimit("test", 10, 0.9, nil)
		asrt.Equal("AIMDLimit{limit=10, backOffRatio=0.9000}", l.String())
	})
}
