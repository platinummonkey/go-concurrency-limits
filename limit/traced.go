package limit

import (
	"fmt"
	"log"

	"github.com/platinummonkey/go-concurrency-limits/core"
)

// Logger implements a basic dependency to log. Feel free to report stats as well.
type Logger interface {
	// Log a Debug statement already formatted.
	Debugf(msg string, params ...interface{})
	// Check if debug is enabled
	IsDebugEnabled() bool
}

// NoopLimitLogger implements a NO-OP logger, it does nothing.
type NoopLimitLogger struct{}

// Debugf debug formatted log
func (l NoopLimitLogger) Debugf(msg string, params ...interface{}) {}

// IsDebugEnabled will return true if debug is enabled. NoopLimitLogger is always `false`
func (l NoopLimitLogger) IsDebugEnabled() bool {
	return false
}

func (l NoopLimitLogger) String() string {
	return "NoopLimitLogger{}"
}

// BuiltinLimitLogger implements a STDOUT limit logger.
type BuiltinLimitLogger struct{}

// Debugf debug formatted log
func (l BuiltinLimitLogger) Debugf(msg string, params ...interface{}) {
	log.Println(fmt.Sprintf(msg, params...))
}

// IsDebugEnabled will return true if debug is enabled. BuiltinLimitLogger is always `true`
func (l BuiltinLimitLogger) IsDebugEnabled() bool {
	return true
}

func (l BuiltinLimitLogger) String() string {
	return "BuiltinLimitLogger{}"
}

// TracedLimit implements core.Limit but adds some additional logging
type TracedLimit struct {
	limit  core.Limit
	logger Logger
}

// NewTracedLimit returns a new wrapped Limit with TracedLimit.
func NewTracedLimit(limit core.Limit, logger Logger) *TracedLimit {
	return &TracedLimit{
		limit:  limit,
		logger: logger,
	}
}

// EstimatedLimit returns the estimated limit.
func (l *TracedLimit) EstimatedLimit() int {
	estimatedLimit := l.limit.EstimatedLimit()
	l.logger.Debugf("estimatedLimit=%d\n", estimatedLimit)
	return estimatedLimit
}

// Update will log and deleate the update of the sample.
func (l *TracedLimit) Update(sample core.SampleWindow) {
	l.logger.Debugf("sampleCount=%d, maxInFlight=%d, minRtt=%d ms",
		sample.SampleCount(), sample.MaxInFlight(), sample.CandidateRTTNanoseconds()/1e6)
	l.limit.Update(sample)
}

func (l TracedLimit) String() string {
	return fmt.Sprintf("TracedLimit{limit=%v, logger=%v}", l.limit, l.logger)
}
