package limit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
}
