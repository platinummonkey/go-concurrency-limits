package limit

import (
	"fmt"
	"log"

	"github.com/platinummonkey/go-concurrency-limits/core"
)

// LimitLogger implements a basic dependency to log. Feel free to report stats as well.
type Logger interface {
	// Log a Debug statement already formatted.
	Debugf(msg string, params ...interface{})
	// Check if debug is enabled
	IsDebugEnabled() bool
}

type NoopLimitLogger struct{}

func (l NoopLimitLogger) Debugf(msg string, params ...interface{}) {}

func (l NoopLimitLogger) IsDebugEnabled() bool {
	return false
}

type BuiltinLimitLogger struct{}

func (l BuiltinLimitLogger) Debugf(msg string, params ...interface{}) {
	log.Println(fmt.Sprintf(msg, params...))
}

func (l BuiltinLimitLogger) IsDebugEnabled() bool {
	return true
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
