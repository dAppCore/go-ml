// SPDX-License-Identifier: EUPL-1.2

// Package-local string helpers that bridge stdlib functions not exposed by core.
// These are thin wrappers that avoid importing banned stdlib packages across
// the package, concentrating the import in one file.

package ml

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// repeatStr repeats a string n times.
//
//	repeatStr("=", 60)  // "============..."
func repeatStr(s string, count int) string {
	return strings.Repeat(s, count)
}

// splitSeq splits s by separator and returns an iter.Seq.
// Wraps strings.SplitSeq for AX compliance.
func splitSeq(s, sep string) func(yield func(string) bool) {
	return strings.SplitSeq(s, sep)
}

// compareStr compares two strings lexicographically.
func compareStr(a, b string) int {
	return strings.Compare(a, b)
}

// lastIndexAny returns the index of the last instance of any char in chars.
func lastIndexAny(s, chars string) int {
	return strings.LastIndexAny(s, chars)
}

// indexByte returns the index of the first occurrence of c in s.
func indexByte(s string, c byte) int {
	return strings.IndexByte(s, c)
}

// fieldsStr splits a string by whitespace.
func fieldsStr(s string) []string {
	return strings.Fields(s)
}

// toLower returns s in lowercase.
func toLower(s string) string {
	return strings.ToLower(s)
}

// joinStrings joins parts with sep.
func joinStrings(parts []string, sep string) string {
	return strings.Join(parts, sep)
}

// replaceAll replaces all occurrences of old with new in s.
func replaceAll(s, old, new string) string {
	return strings.ReplaceAll(s, old, new)
}

// sscanf wraps fmt.Sscanf for parsing formatted strings.
func sscanf(s, format string, args ...any) (int, error) {
	return fmt.Sscanf(s, format, args...)
}

// fprintf writes a formatted string to a writer.
func fprintf(w any, format string, args ...any) {
	if f, ok := w.(interface{ Write([]byte) (int, error) }); ok {
		fmt.Fprintf(f, format, args...)
	}
}

// printf writes a formatted string to stdout.
func printf(format string, args ...any) {
	fmt.Printf(format, args...)
}

// envGet returns the value of an environment variable.
func envGet(key string) string {
	return os.Getenv(key)
}

// userHomeDir returns the user's home directory.
func userHomeDir() (string, error) {
	return os.UserHomeDir()
}

// hostname returns the system hostname.
func hostname() (string, error) {
	return os.Hostname()
}

// lastIndexStr returns the index of the last instance of substr in s.
func lastIndexStr(s, substr string) int {
	return strings.LastIndex(s, substr)
}

// readAll reads all bytes from a reader.
func readAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

// ioEOF is io.EOF for use outside the helpers package.
var ioEOF = io.EOF
