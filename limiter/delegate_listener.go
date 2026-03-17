package limiter

import (
	"github.com/platinummonkey/go-concurrency-limits/core"
)

// DelegateListener wraps a core.Listener and optionally calls an onRelease
// callback after each completion event.  BlockingLimiter uses this to wake
// goroutines that are waiting for a slot to become available.
type DelegateListener struct {
	delegateListener core.Listener
	onRelease        func() // called after every On* method; may be nil
}

// NewDelegateListener creates a new DelegateListener that delegates all calls
// to the wrapped listener without any additional release notification.
func NewDelegateListener(delegateListener core.Listener) *DelegateListener {
	return &DelegateListener{
		delegateListener: delegateListener,
	}
}

// OnDropped is called to indicate the request failed and was dropped due to being rejected by an external limit or
// hitting a timeout.  Loss based Limit implementations will likely do an aggressive reducing in limit when this
// happens.
func (l *DelegateListener) OnDropped() {
	l.delegateListener.OnDropped()
	if l.onRelease != nil {
		l.onRelease()
	}
}

// OnIgnore is called to indicate the operation failed before any meaningful RTT measurement could be made and
// should be ignored to not introduce an artificially low RTT.
func (l *DelegateListener) OnIgnore() {
	l.delegateListener.OnIgnore()
	if l.onRelease != nil {
		l.onRelease()
	}
}

// OnSuccess is called as a notification that the operation succeeded and internally measured latency should be
// used as an RTT sample.
func (l *DelegateListener) OnSuccess() {
	l.delegateListener.OnSuccess()
	if l.onRelease != nil {
		l.onRelease()
	}
}
