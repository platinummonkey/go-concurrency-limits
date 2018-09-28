package measurements

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImmutableSampleWindow(t *testing.T) {
	t.Parallel()
	asrt := assert.New(t)
	w := NewDefaultImmutableSampleWindow()
	asrt.False(w.DidDrop())
	w2 := w.AddSample(10, 10, 5)
	asrt.NotEqual(w, w2)
	asrt.Equal(int64(w2.StartTimeNanoseconds()), w2.StartTimeNanoseconds())
	asrt.Equal(5, w2.MaxInFlight())
	asrt.Equal(1, w2.SampleCount())
	asrt.Equal(int64(10), w2.CandidateRTTNanoseconds())
	asrt.Equal(int64(10), w2.AverageRTTNanoseconds())
	asrt.Equal(
		"ImmutableSampleWindow{minRTT=10, averageRTT=10, maxInFlight=5, sampleCount=1, didDrop=false}",
		w2.String(),
	)
}
