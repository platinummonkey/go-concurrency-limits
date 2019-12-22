package limiter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDelegateListener(t *testing.T) {
	asrt := assert.New(t)
	delegateListener := testListener{}
	listener := NewDelegateListener(&delegateListener)
	listener.OnSuccess()
	asrt.Equal(1, delegateListener.successCount)
	listener.OnIgnore()
	asrt.Equal(1, delegateListener.ignoreCount)
	listener.OnDropped()
	asrt.Equal(1, delegateListener.dropCount)
}
