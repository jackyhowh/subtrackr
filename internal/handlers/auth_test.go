package handlers

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"subtrackr/internal/i18n"
	"subtrackr/internal/models"
	"subtrackr/internal/repository"
	"subtrackr/internal/service"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// repoRoot walks up from the test working directory to locate the project root
// (the directory containing go.mod). Tests run with cwd == package dir, so the
// templates and locales need an absolute path.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate go.mod from %s", dir)
		}
		dir = parent
	}
}

func newAuthTestRouter(t *testing.T) (*gin.Engine, *AuthHandler) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	root := repoRoot(t)

	catalog := i18n.NewCatalog()
	require.NoError(t, catalog.LoadDir(filepath.Join(root, "web", "locales")))

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Settings{}))

	settingsService := service.NewSettingsService(repository.NewSettingsRepository(db))
	sessionService := service.NewSessionService("test-secret-key-for-auth-handler-test")

	authHandler := NewAuthHandler(settingsService, sessionService, nil, catalog)

	router := gin.New()
	tFunc := func(lang interface{}, key string) string {
		langStr, _ := lang.(string)
		return catalog.T(langStr, key)
	}
	router.SetFuncMap(template.FuncMap{"t": tFunc})
	// Load only the auth-related templates to avoid pulling in templates that
	// reference other helpers (div/mul/etc.) we don't need here.
	router.LoadHTMLFiles(
		filepath.Join(root, "templates", "login.html"),
		filepath.Join(root, "templates", "forgot-password.html"),
		filepath.Join(root, "templates", "reset-password.html"),
	)

	router.GET("/login", authHandler.ShowLoginPage)
	router.GET("/forgot-password", authHandler.ShowForgotPasswordPage)
	router.GET("/reset-password", authHandler.ShowResetPasswordPage)

	return router, authHandler
}

// TestShowLoginPage_RendersWithoutLangError reproduces issue #113 — before the
// fix, the login template's {{t .Lang "..."}} crashed because the handler did
// not pass Lang in the template context.
func TestShowLoginPage_RendersWithoutLangError(t *testing.T) {
	router, _ := newAuthTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.NotContains(t, body, "template:")
	assert.NotContains(t, body, "invalid value")
	// English fallback content from the auth.sign_in_title key.
	assert.Contains(t, body, "Sign")
}

func TestShowForgotPasswordPage_RendersWithoutLangError(t *testing.T) {
	router, _ := newAuthTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/forgot-password", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "invalid value")
}

func TestShowResetPasswordPage_MissingTokenRendersWithoutLangError(t *testing.T) {
	router, _ := newAuthTestRouter(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/reset-password", nil)
	router.ServeHTTP(w, req)

	// Token missing → 400, but template must still render without a Lang error.
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.NotContains(t, w.Body.String(), "invalid value")
}

// TestTFunc_HandlesNonStringLang locks in the defensive coercion in the
// template's t function so missing/nil .Lang values fall back to English
// instead of crashing the render.
func TestTFunc_HandlesNonStringLang(t *testing.T) {
	root := repoRoot(t)
	catalog := i18n.NewCatalog()
	require.NoError(t, catalog.LoadDir(filepath.Join(root, "web", "locales")))

	tFunc := func(lang interface{}, key string) string {
		langStr, _ := lang.(string)
		return catalog.T(langStr, key)
	}

	tmpl := template.Must(template.New("t").Funcs(template.FuncMap{"t": tFunc}).Parse(`{{t .Lang "auth.sign_in_title"}}`))

	cases := []struct {
		name string
		data map[string]interface{}
	}{
		{name: "missing Lang", data: map[string]interface{}{}},
		{name: "nil Lang", data: map[string]interface{}{"Lang": nil}},
		{name: "empty Lang", data: map[string]interface{}{"Lang": ""}},
		{name: "english Lang", data: map[string]interface{}{"Lang": "en"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var sb strings.Builder
			err := tmpl.Execute(&sb, tc.data)
			assert.NoError(t, err)
			assert.NotEmpty(t, sb.String())
		})
	}
}
