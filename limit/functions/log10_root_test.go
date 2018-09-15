package functions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLog10RootFunction(t *testing.T) {
	t.Run("ZeroIndex", func(t2 *testing.T) {
		f := Log10RootFunction(4)
		assert.Equal(t2, 4, f(0))
	})

	t.Run("MaxIndex", func(t2 *testing.T) {
		f := Log10RootFunction(4)
		assert.Equal(t2, 31, f(1000))
	})

	t.Run("OutOfLookupRange", func(t2 *testing.T) {
		f := Log10RootFunction(4)
		assert.Equal(t2, 50, f(2500))
	})
}
