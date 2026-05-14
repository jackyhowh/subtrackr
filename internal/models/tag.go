package models

import "time"

// Tag is a free-form label that can be attached to multiple subscriptions, complementing
// the primary Category. The reverse association lives on Subscription.Tags via the
// subscription_tags join table.
type Tag struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"uniqueIndex;not null;size:60"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}
