package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const sampleWallosJSON = `{
  "categories": [
    {"id": 1, "name": "Streaming"},
    {"id": 2, "name": "Productivity"}
  ],
  "currencies": [
    {"id": 1, "name": "US Dollar", "code": "USD"},
    {"id": 2, "name": "Euro", "code": "EUR"}
  ],
  "subscriptions": [
    {
      "name": "Netflix",
      "price": "15.99",
      "currency_id": 1,
      "category_id": 1,
      "cycle": 1,
      "frequency": 3,
      "next_payment": "2026-07-01",
      "notes": "family plan",
      "url": "https://netflix.com",
      "inactive": 0
    },
    {
      "name": "Annual licence",
      "price": "120",
      "currency_id": 2,
      "category_id": 2,
      "cycle": 1,
      "frequency": 4,
      "next_payment": "2027-01-15",
      "inactive": 0
    },
    {
      "name": "Old subscription",
      "price": "5",
      "currency_id": 1,
      "category_id": 1,
      "cycle": 3,
      "frequency": 3,
      "inactive": 1
    },
    {
      "name": "",
      "price": "10",
      "currency_id": 1
    }
  ]
}`

func TestParseWallosBackup(t *testing.T) {
	subs, err := ParseWallosBackup(strings.NewReader(sampleWallosJSON))
	assert.NoError(t, err)
	assert.Len(t, subs, 3, "should skip the unnamed entry")

	// Netflix: USD, Monthly, Active, with category "Streaming"
	netflix := subs[0]
	assert.Equal(t, "Netflix", netflix.Name)
	assert.Equal(t, 15.99, netflix.Cost)
	assert.Equal(t, "USD", netflix.OriginalCurrency)
	assert.Equal(t, "Monthly", netflix.Schedule)
	assert.Equal(t, 1, netflix.ScheduleInterval)
	assert.Equal(t, "Active", netflix.Status)
	assert.Equal(t, "Streaming", netflix.Category.Name)
	assert.NotNil(t, netflix.RenewalDate)

	// Annual licence: EUR, Annual, interval 1, Productivity
	annual := subs[1]
	assert.Equal(t, "EUR", annual.OriginalCurrency)
	assert.Equal(t, "Annual", annual.Schedule)
	assert.Equal(t, 1, annual.ScheduleInterval)
	assert.Equal(t, "Productivity", annual.Category.Name)

	// Old subscription: Monthly cycle=3 should map to Quarterly with interval 1
	quarterly := subs[2]
	assert.Equal(t, "Quarterly", quarterly.Schedule)
	assert.Equal(t, 1, quarterly.ScheduleInterval)
	assert.Equal(t, "Cancelled", quarterly.Status)
}

func TestParseWallosBackup_EmptyReturnsError(t *testing.T) {
	_, err := ParseWallosBackup(strings.NewReader(`{"subscriptions":[]}`))
	assert.Error(t, err)
}

func TestParseWallosBackup_InvalidJSONReturnsError(t *testing.T) {
	_, err := ParseWallosBackup(strings.NewReader(`{not json`))
	assert.Error(t, err)
}
