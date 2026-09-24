//go:build !darwin && !windows

package lang

// systemLocale has nothing to ask beyond the environment, which detect has
// already read.
func systemLocale() string { return "" }
