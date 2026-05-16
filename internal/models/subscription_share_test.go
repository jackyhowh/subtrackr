package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSubscription_ShareCount verifies that ShareCount divides MonthlyCost and AnnualCost,
// that MyShareCost returns the per-period share, and that effectiveShareCount falls back
// to 1 for zero/negative values (defensive against bad DB rows).
func TestSubscription_ShareCount(t *testing.T) {
	tests := []struct {
		name              string
		cost              float64
		schedule          string
		shareCount        int
		wantMonthlyCost   float64
		wantAnnualCost    float64
		wantMyShareCost   float64
		wantIsShared      bool
	}{
		{
			name:            "not shared (default ShareCount=1)",
			cost:            15.99,
			schedule:        "Monthly",
			shareCount:      1,
			wantMonthlyCost: 15.99,
			wantAnnualCost:  15.99 * 12,
			wantMyShareCost: 15.99,
			wantIsShared:    false,
		},
		{
			name:            "shared 4 ways monthly",
			cost:            15.99,
			schedule:        "Monthly",
			shareCount:      4,
			wantMonthlyCost: 15.99 / 4,
			wantAnnualCost:  15.99 * 12 / 4,
			wantMyShareCost: 15.99 / 4,
			wantIsShared:    true,
		},
		{
			name:            "shared 6 ways annual",
			cost:            120,
			schedule:        "Annual",
			shareCount:      6,
			wantMonthlyCost: 120.0 / 12 / 6,
			wantAnnualCost:  120.0 / 6,
			wantMyShareCost: 120.0 / 6,
			wantIsShared:    true,
		},
		{
			name:            "zero ShareCount falls back to 1",
			cost:            10,
			schedule:        "Monthly",
			shareCount:      0,
			wantMonthlyCost: 10,
			wantAnnualCost:  120,
			wantMyShareCost: 10,
			wantIsShared:    false,
		},
		{
			name:            "negative ShareCount falls back to 1",
			cost:            10,
			schedule:        "Monthly",
			shareCount:      -5,
			wantMonthlyCost: 10,
			wantAnnualCost:  120,
			wantMyShareCost: 10,
			wantIsShared:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := &Subscription{
				Cost:       tt.cost,
				Schedule:   tt.schedule,
				ShareCount: tt.shareCount,
			}
			assert.InDelta(t, tt.wantMonthlyCost, sub.MonthlyCost(), 0.001, "MonthlyCost")
			assert.InDelta(t, tt.wantAnnualCost, sub.AnnualCost(), 0.001, "AnnualCost")
			assert.InDelta(t, tt.wantMyShareCost, sub.MyShareCost(), 0.001, "MyShareCost")
			assert.Equal(t, tt.wantIsShared, sub.IsShared(), "IsShared")
		})
	}
}

// TestSubscription_ShareCount_Interval verifies share split composes correctly with
// schedule_interval (e.g. quarterly that bills every 2 quarters, shared 3 ways).
func TestSubscription_ShareCount_Interval(t *testing.T) {
	// Monthly cost = $60, ScheduleInterval = 2 (billed every 2 months), shared 3 ways.
	// Per-billing-period cost is $60, per-month cost is $60/2 = $30, my share is $30/3 = $10.
	sub := &Subscription{
		Cost:             60,
		Schedule:         "Monthly",
		ScheduleInterval: 2,
		ShareCount:       3,
	}
	assert.InDelta(t, 10.0, sub.MonthlyCost(), 0.001)
	assert.InDelta(t, 120.0, sub.AnnualCost(), 0.001) // 12 months * $10
	assert.InDelta(t, 20.0, sub.MyShareCost(), 0.001) // $60 cost / 3 share
}
