package limit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWindowedLimit(t *testing.T) {
	t.Parallel()

	t.Run("DefaultWindowedLimit", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		delegate := NewSettableLimit("test", 10, nil)
		l := NewDefaultWindowedLimit("test", delegate, nil)
		asrt.Equal(10, l.EstimatedLimit())
	})

	t.Run("NewWindowedLimit", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		delegate := NewSettableLimit("test", 10, nil)
		minWindowTime := (time.Millisecond * 100).Nanoseconds()
		l, err := NewWindowedLimit("test", minWindowTime, minWindowTime*2, 10, 10, delegate, nil)
		asrt.NoError(err)
		asrt.NotNil(l)
		asrt.Equal(10, l.EstimatedLimit())
	})

	t.Run("DecreaseOnDrops", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		delegate := NewDefaultAIMDLimit("test", nil)
		minWindowTime := (time.Millisecond * 100).Nanoseconds()
		l, err := NewWindowedLimit("test", minWindowTime, minWindowTime*2, 10, 10, delegate, nil)
		asrt.NoError(err)
		asrt.NotNil(l)
		asrt.Equal(10, l.EstimatedLimit())

		l.OnSample(0, 10, 1, false)
		asrt.Equal(10, l.EstimatedLimit())
		for i := 0; i < 10; i++ {
			l.OnSample(0, minWindowTime*1000, 15, true)
		}
		asrt.Equal(9, l.EstimatedLimit())
	})

	t.Run("IncreaseOnSuccess", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		delegate := NewDefaultAIMDLimit("test", nil)
		minWindowTime := (time.Millisecond * 100).Nanoseconds()
		l, err := NewWindowedLimit("test", minWindowTime, minWindowTime*2, 10, 10, delegate, nil)
		asrt.NoError(err)
		asrt.NotNil(l)
		asrt.Equal(10, l.EstimatedLimit())

		listener := testNotifyListener{}
		l.NotifyOnChange(listener.updater())

		for i := 0; i < 40; i++ {
			l.OnSample(l.minWindowTime*int64(i*i), minWindowTime+10, 15, false)
		}
		asrt.Equal(16, l.EstimatedLimit())
		asrt.Equal([]int{11, 12, 13, 14, 15, 16}, listener.changes)
	})

	t.Run("String", func(t2 *testing.T) {
		t2.Parallel()
		asrt := assert.New(t2)
		delegate := NewSettableLimit("test", 10, nil)
		minWindowTime := (time.Millisecond * 100).Nanoseconds()
		l, err := NewWindowedLimit("test", minWindowTime, minWindowTime*2, 10, 10, delegate, nil)
		asrt.NoError(err)
		asrt.NotNil(l)
		asrt.Equal("WindowedLimit{minWindowTime=100000000, maxWindowTime=200000000, minRTTThreshold=10, "+
			"windowSize=10, delegate=SettableLimit{limit=10}", l.String())
	})
}
