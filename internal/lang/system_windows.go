package lang

import (
	"syscall"
	"unsafe"
)

// systemLocale asks Windows for the user's locale, such as "ja-JP".
func systemLocale() string {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")
	if proc.Find() != nil {
		return ""
	}
	// LOCALE_NAME_MAX_LENGTH is 85.
	buf := make([]uint16, 85)
	n, _, _ := proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}
