package service

import (
	"reflect"
	"testing"
)

func TestParseReminderWindows(t *testing.T) {
	cases := []struct {
		name     string
		csv      string
		fallback int
		want     []int
	}{
		{"empty falls back to single", "", 7, []int{7}},
		{"empty with no fallback returns nil", "", 0, []int{}},
		{"simple list sorted descending", "0,3,7", 0, []int{7, 3, 0}},
		{"dedupe and trim", " 7 , 3, 7 , 0 ", 0, []int{7, 3, 0}},
		{"ignore invalid entries", "abc,5,-1,2", 0, []int{5, 2}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseReminderWindows(tc.csv, tc.fallback)
			if got == nil {
				got = []int{}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ParseReminderWindows(%q, %d) = %v, want %v", tc.csv, tc.fallback, got, tc.want)
			}
		})
	}
}
