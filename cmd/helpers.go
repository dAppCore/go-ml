// SPDX-License-Identifier: EUPL-1.2

// Package-local helpers that bridge stdlib functions not exposed by core.
// Concentrates banned stdlib imports in one file.

package cmd

import (
	"fmt"
	"os"
	"strings"
)

// repeatStr repeats a string n times.
func repeatStr(s string, count int) string {
	return strings.Repeat(s, count)
}

// fieldsStr splits a string by whitespace.
func fieldsStr(s string) []string {
	return strings.Fields(s)
}

// joinStrings joins parts with sep.
func joinStrings(parts []string, sep string) string {
	return strings.Join(parts, sep)
}

// cutStr cuts s around the first instance of sep.
func cutStr(s, sep string) (string, string, bool) {
	return strings.Cut(s, sep)
}

// printf writes a formatted string to stdout.
func printf(format string, args ...any) {
	fmt.Printf(format, args...)
}

// scanln reads a line from stdin.
func scanln(a ...any) (int, error) {
	return fmt.Scanln(a...)
}

// stdinFile returns os.Stdin.
func stdinFile() *os.File {
	return os.Stdin
}

// stdoutFile returns os.Stdout.
func stdoutFile() *os.File {
	return os.Stdout
}

// signalNotify wraps signal.Notify for OS signal handling.
func signalChan() chan os.Signal {
	return make(chan os.Signal, 1)
}
