package service

import (
	"net/http"
	"net/http/httptest"
	"os"
	"subtrackr/internal/models"
	"subtrackr/internal/repository"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Telegram Test Credentials Usage:
//
// For unit tests (default): tests use mock credentials and a stubbed HTTP server.
//
// For integration tests: set environment variables before running tests:
//   export TELEGRAM_BOT_TOKEN="123456:ABC-your-bot-token"
//   export TELEGRAM_CHAT_ID="123456789"
//
// Integration tests automatically skip if credentials are not provided.

func setupTelegramTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	err = db.AutoMigrate(
		&models.Settings{},
		&models.Category{},
	)
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

// newTestTelegramService builds a TelegramService wired to a settings service
// backed by an in-memory DB. Callers can override apiBaseURL/retry knobs.
func newTestTelegramService(t *testing.T) (*TelegramService, *SettingsService) {
	db := setupTelegramTestDB(t)
	settingsRepo := repository.NewSettingsRepository(db)
	settingsService := NewSettingsService(settingsRepo)
	tg := NewTelegramService(settingsService)
	// Keep unit tests fast: no real backoff sleeping between retries.
	tg.initialBackoff = time.Millisecond
	return tg, settingsService
}

// -----------------------------------------------------------------------------
// Unit tests
// -----------------------------------------------------------------------------

func TestTelegramService_SendNotification_NoConfig(t *testing.T) {
	tg, _ := newTestTelegramService(t)

	err := tg.SendNotification("Test", "Test message")
	assert.Error(t, err, "Should return error when Telegram is not configured")
	assert.Contains(t, err.Error(), "Telegram config", "Error should mention Telegram config")
}

func TestTelegramService_SendNotification_EmptyBotToken(t *testing.T) {
	tg, settings := newTestTelegramService(t)

	settings.SaveTelegramConfig(&models.TelegramConfig{BotToken: "", ChatID: "123"})

	err := tg.SendNotification("Test", "Test message")
	assert.Error(t, err, "Should return error when bot token is empty")
	assert.Contains(t, err.Error(), "not configured", "Error should mention not configured")
}

func TestTelegramService_SendNotification_EmptyChatID(t *testing.T) {
	tg, settings := newTestTelegramService(t)

	settings.SaveTelegramConfig(&models.TelegramConfig{BotToken: "token", ChatID: ""})

	err := tg.SendNotification("Test", "Test message")
	assert.Error(t, err, "Should return error when chat ID is empty")
	assert.Contains(t, err.Error(), "not configured", "Error should mention not configured")
}

func TestTelegramService_NetworkError_RedactsBotToken(t *testing.T) {
	tg, settings := newTestTelegramService(t)

	const token = "123456:ABC-secret-token"
	settings.SaveTelegramConfig(&models.TelegramConfig{BotToken: token, ChatID: "123"})

	// Point at a server that is already closed so the transport fails with a
	// *url.Error, whose message embeds the full request URL (token included).
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close()
	tg.apiBaseURL = server.URL
	tg.maxRetries = 1

	err := tg.SendNotification("Test", "Test message")
	assert.Error(t, err, "Should return error when the API is unreachable")
	assert.NotContains(t, err.Error(), token, "Error must not leak the bot token")
	assert.Contains(t, err.Error(), "[REDACTED]", "Token should be redacted in the error")
}

func TestTelegramService_SendHighCostAlert_Disabled(t *testing.T) {
	tg, settings := newTestTelegramService(t)
	settings.SetBoolSetting("high_cost_alerts", false)

	subscription := &models.Subscription{
		Name:     "Test Subscription",
		Cost:     100.00,
		Schedule: "Monthly",
		Status:   "Active",
		Category: models.Category{Name: "Test"},
	}

	err := tg.SendHighCostAlert(subscription)
	assert.NoError(t, err, "Should return nil when high cost alerts are disabled")
}

func TestTelegramService_SendRenewalReminder_Disabled(t *testing.T) {
	tg, settings := newTestTelegramService(t)
	settings.SetBoolSetting("renewal_reminders", false)

	subscription := &models.Subscription{
		Name:        "Test Subscription",
		Cost:        10.00,
		Schedule:    "Monthly",
		Status:      "Active",
		RenewalDate: timePtr(time.Now().AddDate(0, 0, 3)),
		Category:    models.Category{Name: "Test"},
	}

	err := tg.SendRenewalReminder(subscription, 3)
	assert.NoError(t, err, "Should return nil when renewal reminders are disabled")
}

func TestTelegramService_SendCancellationReminder_Disabled(t *testing.T) {
	tg, settings := newTestTelegramService(t)
	settings.SetBoolSetting("cancellation_reminders", false)

	subscription := &models.Subscription{
		Name:             "Test Subscription",
		Cost:             10.00,
		Schedule:         "Monthly",
		Status:           "Active",
		CancellationDate: timePtr(time.Now().AddDate(0, 0, 3)),
		Category:         models.Category{Name: "Test"},
	}

	err := tg.SendCancellationReminder(subscription, 3)
	assert.NoError(t, err, "Should return nil when cancellation reminders are disabled")
}

// -----------------------------------------------------------------------------
// Functional tests — happy path against a stub Telegram API server
// -----------------------------------------------------------------------------

func TestTelegramService_SendNotification_Success(t *testing.T) {
	var gotChatID, gotText, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		gotChatID = r.FormValue("chat_id")
		gotText = r.FormValue("text")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	defer server.Close()

	tg, settings := newTestTelegramService(t)
	tg.apiBaseURL = server.URL
	settings.SaveTelegramConfig(&models.TelegramConfig{BotToken: "bot-token-123", ChatID: "42"})

	err := tg.SendNotification("Hello", "World")
	assert.NoError(t, err, "Should succeed against a healthy Telegram API")
	assert.Equal(t, "42", gotChatID, "chat_id should be forwarded")
	assert.Equal(t, "Hello\n\nWorld", gotText, "title and body should be combined")
	assert.Equal(t, "/botbot-token-123/sendMessage", gotPath, "should call the sendMessage endpoint with the token")
}

func TestTelegramService_SendHighCostAlert_MessageFormat(t *testing.T) {
	var gotText string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		gotText = r.FormValue("text")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	tg, settings := newTestTelegramService(t)
	tg.apiBaseURL = server.URL
	settings.SaveTelegramConfig(&models.TelegramConfig{BotToken: "t", ChatID: "1"})
	settings.SetBoolSetting("high_cost_alerts", true)
	settings.SetCurrency("USD")

	subscription := &models.Subscription{
		Name:        "Netflix",
		Cost:        15.99,
		Schedule:    "Monthly",
		Status:      "Active",
		RenewalDate: timePtr(time.Now().AddDate(0, 0, 30)),
		Category:    models.Category{Name: "Entertainment"},
		URL:         "https://netflix.com",
	}

	err := tg.SendHighCostAlert(subscription)
	assert.NoError(t, err)
	assert.Contains(t, gotText, "High Cost Alert", "Message should reference the alert type")
	assert.Contains(t, gotText, "Netflix", "Message should include the subscription name")
	assert.Contains(t, gotText, "15.99", "Message should include the cost")
}

func TestTelegramService_APIError_NotRetried(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest) // 400: client error, not retryable
		w.Write([]byte(`{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`))
	}))
	defer server.Close()

	tg, settings := newTestTelegramService(t)
	tg.apiBaseURL = server.URL
	settings.SaveTelegramConfig(&models.TelegramConfig{BotToken: "t", ChatID: "1"})

	err := tg.SendNotification("Test", "message")
	assert.Error(t, err, "A 400 should surface as an error")
	assert.Contains(t, err.Error(), "chat not found", "Error should carry the API description")
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "Non-retryable errors must not be retried")
}

// -----------------------------------------------------------------------------
// Retry tests — external Telegram/HTTP calls retry with exponential backoff
// -----------------------------------------------------------------------------

func TestTelegramService_RetriesOn5xxThenSucceeds(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"ok":false,"error_code":500,"description":"server error"}`))
			return
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	tg, settings := newTestTelegramService(t)
	tg.apiBaseURL = server.URL
	tg.maxRetries = 3
	settings.SaveTelegramConfig(&models.TelegramConfig{BotToken: "t", ChatID: "1"})

	err := tg.SendNotification("Test", "message")
	assert.NoError(t, err, "Should succeed after transient 5xx failures are retried")
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls), "Should have retried until success")
}

func TestTelegramService_RetriesExhausted(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusTooManyRequests) // 429 is retryable
		w.Write([]byte(`{"ok":false,"error_code":429,"description":"Too Many Requests"}`))
	}))
	defer server.Close()

	tg, settings := newTestTelegramService(t)
	tg.apiBaseURL = server.URL
	tg.maxRetries = 3
	settings.SaveTelegramConfig(&models.TelegramConfig{BotToken: "t", ChatID: "1"})

	err := tg.SendNotification("Test", "message")
	assert.Error(t, err, "Should return an error once retries are exhausted")
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls), "Should attempt exactly maxRetries times")
}

func TestTelegramService_BackoffIsExponential(t *testing.T) {
	var times []time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		times = append(times, time.Now())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"ok":false}`))
	}))
	defer server.Close()

	tg, settings := newTestTelegramService(t)
	tg.apiBaseURL = server.URL
	tg.maxRetries = 3
	tg.initialBackoff = 20 * time.Millisecond
	settings.SaveTelegramConfig(&models.TelegramConfig{BotToken: "t", ChatID: "1"})

	_ = tg.SendNotification("Test", "message")

	if assert.Len(t, times, 3, "Should have made 3 attempts") {
		gap1 := times[1].Sub(times[0])
		gap2 := times[2].Sub(times[1])
		assert.GreaterOrEqual(t, gap1, 15*time.Millisecond, "First backoff should be ~initialBackoff")
		assert.Greater(t, gap2, gap1, "Second backoff should be larger than the first (exponential)")
	}
}

// -----------------------------------------------------------------------------
// Frame tests — table-driven boundary/pluralization behaviour
// -----------------------------------------------------------------------------

func TestTelegramService_RenewalReminder_DaysText(t *testing.T) {
	var gotText string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		gotText = r.FormValue("text")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	tg, settings := newTestTelegramService(t)
	tg.apiBaseURL = server.URL
	settings.SaveTelegramConfig(&models.TelegramConfig{BotToken: "t", ChatID: "1"})
	settings.SetBoolSetting("renewal_reminders", true)
	settings.SetCurrency("USD")

	subscription := &models.Subscription{
		Name:        "Test Subscription",
		Cost:        10.00,
		Schedule:    "Monthly",
		Status:      "Active",
		RenewalDate: timePtr(time.Now().AddDate(0, 0, 1)),
		Category:    models.Category{Name: "Test"},
	}

	tests := []struct {
		name       string
		daysUntil  int
		wantSubstr string
	}{
		{name: "Singular day", daysUntil: 1, wantSubstr: "in 1 day."},
		{name: "Plural days", daysUntil: 3, wantSubstr: "in 3 days."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tg.SendRenewalReminder(subscription, tt.daysUntil)
			assert.NoError(t, err)
			assert.Contains(t, gotText, tt.wantSubstr, "Message should use correct day pluralization")
		})
	}
}

// -----------------------------------------------------------------------------
// Security tests
// -----------------------------------------------------------------------------

// GetTelegramConfig (handler-facing) must never leak the full bot token; here we
// verify the service layer keeps the token confined to the request path/body and
// that empty/whitespace credentials are rejected before any network call.
func TestTelegramService_Security_NoSendWhenUnconfigured(t *testing.T) {
	var called int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&called, 1)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	tg, settings := newTestTelegramService(t)
	tg.apiBaseURL = server.URL
	// Missing chat ID: must fail fast without contacting the network.
	settings.SaveTelegramConfig(&models.TelegramConfig{BotToken: "secret-token", ChatID: ""})

	err := tg.SendNotification("Test", "message")
	assert.Error(t, err)
	assert.Equal(t, int32(0), atomic.LoadInt32(&called), "Must not make a network call when unconfigured")
	assert.NotContains(t, err.Error(), "secret-token", "Error message must not leak the bot token")
}

// -----------------------------------------------------------------------------
// Performance tests
// -----------------------------------------------------------------------------

// Ensure a healthy send returns promptly and that the client enforces a timeout
// so a hung Telegram endpoint cannot block the reminder scheduler indefinitely.
func TestTelegramService_Performance_FastPathAndTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	tg, settings := newTestTelegramService(t)
	tg.apiBaseURL = server.URL
	settings.SaveTelegramConfig(&models.TelegramConfig{BotToken: "t", ChatID: "1"})

	start := time.Now()
	err := tg.SendNotification("Test", "message")
	assert.NoError(t, err)
	assert.Less(t, time.Since(start), 2*time.Second, "Healthy send should return quickly")

	// The service must configure a bounded HTTP client timeout (defense against hangs).
	assert.Positive(t, tg.httpClient.Timeout, "HTTP client must have a non-zero timeout")
	assert.LessOrEqual(t, tg.httpClient.Timeout, 30*time.Second, "HTTP client timeout should be bounded")
}

// -----------------------------------------------------------------------------
// Integration tests (opt-in via environment variables)
// -----------------------------------------------------------------------------

func getTelegramTestCredentials() (botToken, chatID string) {
	return os.Getenv("TELEGRAM_BOT_TOKEN"), os.Getenv("TELEGRAM_CHAT_ID")
}

func TestTelegramService_SendNotification_Integration(t *testing.T) {
	botToken, chatID := getTelegramTestCredentials()
	if botToken == "" || chatID == "" {
		t.Skip("Skipping integration test: TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID environment variables not set")
	}

	tg, settings := newTestTelegramService(t)
	// Use the real Telegram API for the integration path.
	tg.apiBaseURL = telegramAPIBaseURL
	err := settings.SaveTelegramConfig(&models.TelegramConfig{BotToken: botToken, ChatID: chatID})
	assert.NoError(t, err, "Should save Telegram config")

	err = tg.SendNotification("SubTrackr Test", "This is a test notification from SubTrackr integration tests")
	assert.NoError(t, err, "Should successfully send notification with valid credentials")
}

func TestTelegramService_SendRenewalReminder_Integration(t *testing.T) {
	botToken, chatID := getTelegramTestCredentials()
	if botToken == "" || chatID == "" {
		t.Skip("Skipping integration test: TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID environment variables not set")
	}

	tg, settings := newTestTelegramService(t)
	tg.apiBaseURL = telegramAPIBaseURL
	settings.SaveTelegramConfig(&models.TelegramConfig{BotToken: botToken, ChatID: chatID})
	settings.SetBoolSetting("renewal_reminders", true)
	settings.SetCurrency("USD")

	subscription := &models.Subscription{
		Name:        "Test Subscription",
		Cost:        15.99,
		Schedule:    "Monthly",
		Status:      "Active",
		RenewalDate: timePtr(time.Now().AddDate(0, 0, 3)),
		Category:    models.Category{Name: "Test"},
		URL:         "https://example.com",
	}

	err := tg.SendRenewalReminder(subscription, 3)
	assert.NoError(t, err, "Should successfully send renewal reminder with valid credentials")
}
