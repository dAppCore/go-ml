// SPDX-License-Identifier: EUPL-1.2

// Package-local helpers that bridge stdlib functions not exposed by core.
// Concentrates banned stdlib imports in one file.

package cmd

import (
	core "dappco.re/go"
)

// repeatStr repeats a string n times.
func repeatStr(s string, count int) string {
	if count <= 0 || s == "" {
		return ""
	}
	b := core.NewBuilder()
	for range count {
		b.WriteString(s)
	}
	return b.String()
}

// fieldsStr splits a string by whitespace.
func fieldsStr(s string) []string {
	var fields []string
	start := -1
	for i := range len(s) {
		if isSpaceByte(s[i]) {
			if start >= 0 {
				fields = append(fields, s[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		fields = append(fields, s[start:])
	}
	return fields
}

// joinStrings joins parts with sep.
func joinStrings(parts []string, sep string) string {
	return core.Join(sep, parts...)
}

// cutStr cuts s around the first instance of sep.
func cutStr(s, sep string) (string, string, bool) {
	idx := indexSubstr(s, sep)
	if idx < 0 {
		return s, "", false
	}
	return s[:idx], s[idx+len(sep):], true
}

// printf writes a formatted string to stdout.
func printf(format string, args ...any) {
	core.WriteString(core.Stdout(), core.Sprintf(format, args...))
}

func indexSubstr(s, substr string) int {
	if substr == "" {
		return 0
	}
	if len(substr) > len(s) {
		return -1
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func isSpaceByte(b byte) bool {
	switch b {
	case ' ', '\n', '\r', '\t', '\v', '\f':
		return true
	default:
		return false
	}
}
