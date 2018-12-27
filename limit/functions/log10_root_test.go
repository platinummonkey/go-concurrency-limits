package functions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLog10RootFunction(t *testing.T) {
	t.Parallel()

	t.Run("ZeroIndex", func(t2 *testing.T) {
		t2.Parallel()
		f := Log10RootFunction(4)
		assert.Equal(t2, 4, f(0))
	})

	t.Run("MaxIndex", func(t2 *testing.T) {
		t2.Parallel()
		f := Log10RootFunction(4)
		assert.Equal(t2, 5, f(100000))
	})

	t.Run("OutOfLookupRange", func(t2 *testing.T) {
		t2.Parallel()
		f := Log10RootFunction(4)
		assert.Equal(t2, 5, f(250000))
	})
}

func TestLog10RootFloatFunction(t *testing.T) {
	t.Parallel()

	t.Run("ZeroIndex", func(t2 *testing.T) {
		t2.Parallel()
		f := Log10RootFloatFunction(4)
		assert.Equal(t2, 4.0, f(0))
	})

	t.Run("MaxIndex", func(t2 *testing.T) {
		t2.Parallel()
		f := Log10RootFloatFunction(4)
		assert.InDelta(t2, 5, f(100000), 0.001)
	})

	t.Run("OutOfLookupRange", func(t2 *testing.T) {
		t2.Parallel()
		f := Log10RootFloatFunction(4)
		assert.InDelta(t2, 5.3979, f(250000), 0.001)
	})
}
