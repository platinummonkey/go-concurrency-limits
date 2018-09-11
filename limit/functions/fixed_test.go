package functions

import (
	"github.com/magiconair/properties/assert"
	"testing"
)

func TestFixedQueueSizeFunc(t *testing.T) {
	f := FixedQueueSizeFunc(4)
	for i := -4; i < 5; i++ {
		assert.Equal(t, 4, f(0))
	}
}
