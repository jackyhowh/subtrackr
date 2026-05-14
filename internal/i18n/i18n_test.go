package i18n

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCatalogLoadAndLookup(t *testing.T) {
	dir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "en.json"), []byte(`{"lang.name":"English","hello":"Hello"}`), 0644))
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "es.json"), []byte(`{"lang.name":"Español","hello":"Hola"}`), 0644))

	c := NewCatalog()
	assert.NoError(t, c.LoadDir(dir))

	assert.Equal(t, "Hello", c.T("en", "hello"))
	assert.Equal(t, "Hola", c.T("es", "hello"))

	// Missing key falls back to English
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "de.json"), []byte(`{"lang.name":"Deutsch"}`), 0644))
	c2 := NewCatalog()
	assert.NoError(t, c2.LoadDir(dir))
	assert.Equal(t, "Hello", c2.T("de", "hello"))

	// Unknown lang falls back to English
	assert.Equal(t, "Hello", c.T("fr", "hello"))

	// Unknown key returns key as-is
	assert.Equal(t, "missing.key", c.T("en", "missing.key"))

	langs := c2.AvailableLanguages()
	assert.GreaterOrEqual(t, len(langs), 3)
}
