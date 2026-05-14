// Package i18n provides a minimal translation catalog for SubTrackr's UI strings.
// Translations live as flat-key JSON files in web/locales/<code>.json (e.g. en.json).
// Each language file is a simple map of dotted-key → translated string. English serves
// as the canonical source and the fallback for missing keys in other languages.
package i18n

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const fallbackLang = "en"

// Catalog holds loaded translations keyed by language code.
type Catalog struct {
	mu        sync.RWMutex
	languages map[string]map[string]string
}

// Language describes an available UI language for selectors and the API.
type Language struct {
	Code string `json:"code"` // ISO 639-1 (e.g. "en", "es")
	Name string `json:"name"` // Display name in that language (from lang.name key)
}

func NewCatalog() *Catalog {
	return &Catalog{languages: make(map[string]map[string]string)}
}

// LoadDir loads every *.json file in dir into the catalog. The filename (minus
// extension) is used as the language code.
func (c *Catalog) LoadDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read locales dir %q: %w", dir, err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		code := strings.TrimSuffix(name, ".json")

		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		msgs := make(map[string]string)
		if err := json.Unmarshal(data, &msgs); err != nil {
			return fmt.Errorf("parse %s: %w", name, err)
		}
		c.languages[code] = msgs
	}
	return nil
}

// T looks up the translation for key in the given language, falling back to English,
// then to the key itself if nothing is found. Safe to call from templates.
func (c *Catalog) T(lang, key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if msgs, ok := c.languages[lang]; ok {
		if v, ok := msgs[key]; ok && v != "" {
			return v
		}
	}
	if msgs, ok := c.languages[fallbackLang]; ok {
		if v, ok := msgs[key]; ok && v != "" {
			return v
		}
	}
	return key
}

// AvailableLanguages returns the loaded languages sorted by code, each with its
// display name (the lang.name key in that language file).
func (c *Catalog) AvailableLanguages() []Language {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]Language, 0, len(c.languages))
	for code, msgs := range c.languages {
		name := msgs["lang.name"]
		if name == "" {
			name = code
		}
		out = append(out, Language{Code: code, Name: name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

// HasLanguage reports whether a translation file exists for the given language code.
func (c *Catalog) HasLanguage(code string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, ok := c.languages[code]
	return ok
}
