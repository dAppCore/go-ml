// SPDX-License-Identifier: EUPL-1.2

// Package-local string helpers that bridge stdlib functions not exposed by core.
// These are thin wrappers that avoid importing banned stdlib packages across
// the package, concentrating the import in one file.

package ml

import (
	core "dappco.re/go"
)

// repeatStr repeats a string n times.
//
//	repeatStr("=", 60)  // "============..."
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

// splitSeq splits s by separator and returns an iter.Seq.
// Wraps strings.SplitSeq for AX compliance.
func splitSeq(s, sep string) func(yield func(string) bool) {
	parts := core.Split(s, sep)
	return func(yield func(string) bool) {
		for _, part := range parts {
			if !yield(part) {
				return
			}
		}
	}
}

// compareStr compares two strings lexicographically.
func compareStr(a, b string) int {
	return core.Compare(a, b)
}

// lastIndexAny returns the index of the last instance of any char in chars.
func lastIndexAny(s, chars string) int {
	for i := len(s) - 1; i >= 0; i-- {
		for j := 0; j < len(chars); j++ {
			if s[i] == chars[j] {
				return i
			}
		}
	}
	return -1
}

// indexByte returns the index of the first occurrence of c in s.
func indexByte(s string, c byte) int {
	for i := range len(s) {
		if s[i] == c {
			return i
		}
	}
	return -1
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

// toLower returns s in lowercase.
func toLower(s string) string {
	return core.Lower(s)
}

// joinStrings joins parts with sep.
func joinStrings(parts []string, sep string) string {
	return core.Join(sep, parts...)
}

// replaceAll replaces all occurrences of old with new in s.
func replaceAll(s, old, new string) string {
	return core.Replace(s, old, new)
}

// applyStopSequences truncates text at the first occurrence of any stop
// sequence. Empty stop sequences are ignored.
func applyStopSequences(text string, stopSequences []string) string {
	if text == "" || len(stopSequences) == 0 {
		return text
	}

	cut := len(text)
	for _, stop := range stopSequences {
		if stop == "" {
			continue
		}
		if idx := indexSubstr(text, stop); idx >= 0 && idx < cut {
			cut = idx
		}
	}

	return text[:cut]
}

// isErrorResponse reports whether the response should be treated as an
// error prefix regardless of case or leading whitespace.
func isErrorResponse(s string) bool {
	return core.HasPrefix(core.Lower(core.Trim(s)), "error")
}

// fprintf writes a formatted string to a writer.
func fprintf(w any, format string, args ...any) {
	if f, ok := w.(interface{ Write([]byte) (int, error) }); ok {
		_, _ = f.Write([]byte(core.Sprintf(format, args...)))
	}
}

// printf writes a formatted string to stdout.
func printf(format string, args ...any) {
	core.WriteString(core.Stdout(), core.Sprintf(format, args...))
}

// envGet returns the value of an environment variable.
func envGet(key string) string {
	return core.Getenv(key)
}

// userHomeDir returns the user's home directory.
//
//	r := userHomeDir()
//	if !r.OK { return r }
//	home := r.Value.(string)
func userHomeDir() core.Result {
	return core.UserHomeDir()
}

// hostname returns the system hostname.
//
//	r := hostname()
//	if !r.OK { return r }
//	host := r.Value.(string)
func hostname() core.Result {
	return core.Hostname()
}

// lastIndexStr returns the index of the last instance of substr in s.
func lastIndexStr(s, substr string) int {
	last := -1
	for offset := 0; offset <= len(s)-len(substr); offset++ {
		if substr == "" || s[offset:offset+len(substr)] == substr {
			last = offset
		}
	}
	return last
}

// readAll reads all bytes from a reader.
//
//	r := readAll(resp.Body)
//	if !r.OK { return r }
//	data := r.Value.([]byte)
func readAll(r any) core.Result {
	result := core.ReadAll(r)
	if !result.OK {
		return result
	}
	return core.Ok([]byte(result.Value.(string)))
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
