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
		assert.Equal(t2, 5, f(0))
	})

	t.Run("MaxIndex", func(t2 *testing.T) {
		t2.Parallel()
		f := Log10RootFunction(4)
		assert.Equal(t2, 7, f(1000))
	})

	t.Run("OutOfLookupRange", func(t2 *testing.T) {
		t2.Parallel()
		f := Log10RootFunction(4)
		assert.Equal(t2, 7, f(2500))
	})
}

func TestLog10RootFloatFunction(t *testing.T) {
	t.Parallel()

	t.Run("ZeroIndex", func(t2 *testing.T) {
		t2.Parallel()
		f := Log10RootFloatFunction(4)
		assert.Equal(t2, 5.0, f(0))
	})

	t.Run("MaxIndex", func(t2 *testing.T) {
		t2.Parallel()
		f := Log10RootFloatFunction(4)
		assert.InDelta(t2, 7.0, f(1000), 0.001)
	})

	t.Run("OutOfLookupRange", func(t2 *testing.T) {
		t2.Parallel()
		f := Log10RootFloatFunction(4)
		assert.Equal(t2, 7.3979400086720375, f(2500))
	})
}
