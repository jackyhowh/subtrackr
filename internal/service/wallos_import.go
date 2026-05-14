package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"subtrackr/internal/models"
	"time"
)

// Wallos (https://github.com/ellite/Wallos) is another self-hosted subscription tracker.
// Its backup JSON is a top-level object containing arrays for subscriptions, categories,
// currencies, payment_methods, etc. The schema is loosely documented and has evolved over
// time, so this importer is intentionally lenient: it pulls what it can map cleanly to
// SubTrackr's model and ignores fields without a clean equivalent.

// wallosBackup mirrors the relevant top-level fields from a Wallos export.
type wallosBackup struct {
	Subscriptions []wallosSubscription `json:"subscriptions"`
	Categories    []wallosCategory     `json:"categories"`
	Currencies    []wallosCurrency     `json:"currencies"`
}

type wallosCategory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type wallosCurrency struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

// wallosSubscription handles the fields Wallos exports per subscription. Some numeric
// fields ship as strings in older exports, so we accept json.Number for tolerance.
type wallosSubscription struct {
	Name        string      `json:"name"`
	Price       json.Number `json:"price"`
	CurrencyID  int         `json:"currency_id"`
	CategoryID  int         `json:"category_id"`
	Cycle       int         `json:"cycle"`     // count of frequency units
	Frequency   int         `json:"frequency"` // 1=days 2=weeks 3=months 4=years
	NextPayment string      `json:"next_payment"`
	Notes       string      `json:"notes"`
	URL         string      `json:"url"`
	Inactive    int         `json:"inactive"`
}

// ImportWallosResult summarises the outcome of an import for the response payload.
type ImportWallosResult struct {
	Imported int      `json:"imported"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
}

// ParseWallosBackup parses a Wallos JSON export from the given reader into SubTrackr
// subscriptions. Categories are returned as embedded names; the caller is responsible
// for resolving them against the SubTrackr Category table (matching existing restore flow).
func ParseWallosBackup(r io.Reader) ([]models.Subscription, error) {
	decoder := json.NewDecoder(r)
	decoder.UseNumber()

	var backup wallosBackup
	if err := decoder.Decode(&backup); err != nil {
		return nil, fmt.Errorf("invalid Wallos backup JSON: %w", err)
	}

	if len(backup.Subscriptions) == 0 {
		return nil, errors.New("Wallos backup contains no subscriptions")
	}

	categoryByID := make(map[int]string, len(backup.Categories))
	for _, c := range backup.Categories {
		categoryByID[c.ID] = c.Name
	}

	currencyByID := make(map[int]string, len(backup.Currencies))
	for _, c := range backup.Currencies {
		code := strings.ToUpper(strings.TrimSpace(c.Code))
		if code == "" {
			// Some Wallos exports only ship Name; try mapping common names.
			code = strings.ToUpper(strings.TrimSpace(c.Name))
		}
		if len(code) == 3 {
			currencyByID[c.ID] = code
		}
	}

	out := make([]models.Subscription, 0, len(backup.Subscriptions))
	for _, w := range backup.Subscriptions {
		sub, err := walloSubToModel(w, categoryByID, currencyByID)
		if err != nil {
			// Skip rows we can't translate at all, but keep the import flowing.
			continue
		}
		out = append(out, sub)
	}
	return out, nil
}

func walloSubToModel(w wallosSubscription, categories map[int]string, currencies map[int]string) (models.Subscription, error) {
	name := strings.TrimSpace(w.Name)
	if name == "" {
		return models.Subscription{}, errors.New("missing name")
	}

	cost, err := strconv.ParseFloat(w.Price.String(), 64)
	if err != nil || cost < 0 {
		return models.Subscription{}, fmt.Errorf("invalid price %q", w.Price.String())
	}

	schedule, interval := wallosCycleToSchedule(w.Frequency, w.Cycle)

	currency := currencies[w.CurrencyID]
	if currency == "" {
		currency = "USD"
	}

	status := "Active"
	if w.Inactive == 1 {
		status = "Cancelled"
	}

	var renewalDate *time.Time
	if w.NextPayment != "" {
		if t, err := time.Parse("2006-01-02", w.NextPayment); err == nil {
			renewalDate = &t
		}
	}

	sub := models.Subscription{
		Name:             name,
		Cost:             cost,
		OriginalCurrency: currency,
		Schedule:         schedule,
		ScheduleInterval: interval,
		Status:           status,
		URL:              strings.TrimSpace(w.URL),
		Notes:            strings.TrimSpace(w.Notes),
		RenewalDate:      renewalDate,
		ShareCount:       1,
		ReminderEnabled:  true,
	}
	if catName := strings.TrimSpace(categories[w.CategoryID]); catName != "" {
		sub.Category = models.Category{Name: catName}
	}
	return sub, nil
}

// wallosCycleToSchedule maps Wallos's (frequency, cycle) pair into SubTrackr's
// (schedule string, schedule interval int). Frequency: 1=days, 2=weeks, 3=months, 4=years.
func wallosCycleToSchedule(frequency, cycle int) (string, int) {
	if cycle <= 0 {
		cycle = 1
	}
	switch frequency {
	case 1:
		return "Daily", cycle
	case 2:
		return "Weekly", cycle
	case 4:
		return "Annual", cycle
	default: // 3 (months) or unknown
		if cycle == 3 {
			return "Quarterly", 1
		}
		return "Monthly", cycle
	}
}
