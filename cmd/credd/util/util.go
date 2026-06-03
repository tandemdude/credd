package util

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// PromptString writes a labelled prompt (showing def when non-empty) and returns
// the trimmed input, or def when the input is blank.
func PromptString(scanner *bufio.Scanner, out io.Writer, label, def string) string {
	if def != "" {
		fmt.Fprintf(out, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(out, "%s: ", label)
	}
	if !scanner.Scan() {
		return def
	}
	v := strings.TrimSpace(scanner.Text())
	if v == "" {
		return def
	}
	return v
}

// ScanYes reads one line and returns true for an affirmative answer (y/yes).
func ScanYes(scanner *bufio.Scanner) bool {
	if !scanner.Scan() {
		return false
	}
	a := strings.TrimSpace(scanner.Text())
	return strings.EqualFold(a, "y") || strings.EqualFold(a, "yes")
}

// Confirm prints prompt to stderr and returns true for an affirmative answer
// (y/yes, case-insensitive).
func Confirm(prompt string) bool {
	fmt.Fprint(os.Stderr, prompt)
	return ScanYes(bufio.NewScanner(os.Stdin))
}
