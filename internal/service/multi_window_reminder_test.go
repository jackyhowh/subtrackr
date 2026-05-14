package service

import (
	"subtrackr/internal/models"
	"subtrackr/internal/repository"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestMultiWindow_FiresAtEachThreshold covers the new multi-window behavior: a sub with
// windows [7,3,0] should be returned each time daysUntil crosses below a new threshold,
// and skipped after each threshold has fired.
func TestMultiWindow_FiresAtEachThreshold(t *testing.T) {
	db := setupRenewalReminderTestDB(t)
	subRepo := repository.NewSubscriptionRepository(db)
	catRepo := repository.NewCategoryRepository(db)
	catSvc := NewCategoryService(catRepo)
	svc := NewSubscriptionService(subRepo, catSvc)

	now := time.Now()
	renewal := now.AddDate(0, 0, 5) // renews in 5 days

	sub := &models.Subscription{
		Name:               "MultiWindow",
		Cost:               9.99,
		Schedule:           "Monthly",
		Status:             "Active",
		RenewalDate:        &renewal,
		ReminderEnabled:    true,
		LastReminderWindow: -1,
	}
	assert.NoError(t, db.Create(sub).Error)

	// First check at daysUntil=5 with windows [7,3,0]: smallest matching window >= 5 is 7. Fire.
	result, err := svc.GetSubscriptionsNeedingReminders([]int{7, 3, 0})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(result), "should fire 7-day reminder when 5 days out")

	// Simulate marking the 5-day fire (LastReminderWindow=5, LastReminderSent set, same renewal date).
	sent := now
	sub.LastReminderSent = &sent
	sub.LastReminderRenewalDate = &renewal
	sub.LastReminderWindow = 5
	assert.NoError(t, db.Save(sub).Error)

	// Same daysUntil — should now skip because we already fired 5 (<=7).
	result, err = svc.GetSubscriptionsNeedingReminders([]int{7, 3, 0})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(result), "should skip after 7-day window already fired")

	// Move renewal closer to 2 days: matched window = 3. LastReminderWindow=5 > 3 → fire again.
	closer := now.AddDate(0, 0, 2)
	sub.RenewalDate = &closer
	// LastReminderRenewalDate still points to old date so sameRenewal becomes false; reset window tracker.
	sub.LastReminderWindow = -1
	sub.LastReminderRenewalDate = nil
	assert.NoError(t, db.Save(sub).Error)

	result, err = svc.GetSubscriptionsNeedingReminders([]int{7, 3, 0})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(result), "should fire 3-day reminder when 2 days out")
}
