// Package winres holds the icon compiled into gumpet.exe.
//
// The window already gets an icon at run time, made from the pet in use (see
// internal/icon). That one only exists while the window does. Explorer, a
// taskbar pin and the moment before the window opens all look at the
// executable instead, and an executable with no icon resource is shown with
// Windows' blank placeholder. Ebitengine's GLFW port does not read a GLFW_ICON
// resource either, so nothing would fill that gap.
//
// The icons are made from the default gopher by the same code as the window's,
// so the two match. The PNGs here are generated, and the .syso files beside
// cmd/gumpet's main.go are built from them; regenerate both with
//
//	go generate ./cmd/gumpet
//
// The test in this package fails if the PNGs no longer match the gopher.
package winres
