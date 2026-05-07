package quota

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEvalThreshold drives the rising-edge + hysteresis function with the
// canonical sequence from the Phase 4 plan: (50,75,81,85,70,82) at threshold
// 80 with 10pp hysteresis. Alerts must fire only at indexes 2 and 5.
func TestEvalThreshold_CanonicalSequence(t *testing.T) {
	const threshold = 80
	const hysteresis = 10

	type step struct {
		current      int
		wantFire     bool
		wantReset    bool
		afterLastPct int // expected lastPercent after this step
	}

	steps := []step{
		{current: 50, wantFire: false, wantReset: true, afterLastPct: 0},
		{current: 75, wantFire: false, wantReset: false, afterLastPct: 0},
		{current: 81, wantFire: true, wantReset: false, afterLastPct: 81},
		{current: 85, wantFire: false, wantReset: false, afterLastPct: 81},
		{current: 70, wantFire: false, wantReset: true, afterLastPct: 0},
		{current: 82, wantFire: true, wantReset: false, afterLastPct: 82},
	}

	lastPercent := 0
	for i, s := range steps {
		fire, reset := evalThreshold(lastPercent, threshold, s.current, hysteresis)
		assert.Equal(t, s.wantFire, fire, "step %d (current=%d): fire", i, s.current)
		assert.Equal(t, s.wantReset, reset, "step %d (current=%d): reset", i, s.current)

		// Simulate state update.
		if fire {
			lastPercent = s.current
		} else if reset {
			lastPercent = 0
		}
		assert.Equal(t, s.afterLastPct, lastPercent, "step %d: lastPercent after", i)
	}
}

func TestEvalThreshold_NoAlert_BelowThreshold(t *testing.T) {
	fire, reset := evalThreshold(0, 80, 50, 10)
	assert.False(t, fire)
	assert.True(t, reset) // 50 ≤ 80-10=70
}

func TestEvalThreshold_NoAlert_AlreadyCrossed(t *testing.T) {
	// last=85, threshold=80, current=90 — already crossed, no new alert
	fire, reset := evalThreshold(85, 80, 90, 10)
	assert.False(t, fire)
	assert.False(t, reset)
}

func TestEvalThreshold_ExactBoundary(t *testing.T) {
	// current exactly equals threshold
	fire, reset := evalThreshold(0, 80, 80, 10)
	assert.True(t, fire)
	assert.False(t, reset)

	// current exactly at hysteresis boundary (threshold-hysteresis)
	fire, reset = evalThreshold(85, 80, 70, 10)
	assert.False(t, fire)
	assert.True(t, reset) // 70 ≤ 70
}
