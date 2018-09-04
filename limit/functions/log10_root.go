package functions

import (
	"math"
)

var log10RootLookup []int

// Log10RootFunction is a specialized utility function used by limiters to calculate thresholds using log10 of the
// current limit.  Here we pre-compute the log10 root of numbers up to 1000 (or more) because the log10 root
// operation can be slow.
func Log10RootFunction(baseline int) func(estimatedLimit int) int {
	return func(estimatedLimit int) int {
		if estimatedLimit < len(log10RootLookup) {
			return max(baseline, log10RootLookup[estimatedLimit])
		}
		return max(baseline, int(math.Sqrt(float64(estimatedLimit))))
	}
}
