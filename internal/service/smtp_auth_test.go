package service

import (
	"reflect"
	"testing"
)

func TestParseEmailRecipients(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", []string{}},
		{"whitespace only", "   ", []string{}},
		{"single", "a@example.com", []string{"a@example.com"}},
		{"single trimmed", "  a@example.com  ", []string{"a@example.com"}},
		{"comma separated", "a@example.com,b@example.com", []string{"a@example.com", "b@example.com"}},
		{"semicolon separated", "a@example.com;b@example.com", []string{"a@example.com", "b@example.com"}},
		{"mixed with spaces", "a@example.com; b@example.com , c@example.com", []string{"a@example.com", "b@example.com", "c@example.com"}},
		{"trailing separators", "a@example.com;;,", []string{"a@example.com"}},
		{"empty between separators", "a@example.com,,b@example.com", []string{"a@example.com", "b@example.com"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseEmailRecipients(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseEmailRecipients(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}
