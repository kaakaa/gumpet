// Package winres holds the icon compiled into gumpet.exe.
//
// The window gets an icon at run time, made from the pet in use (see
// internal/icon), but that one only exists while the window does. Explorer, a
// taskbar pin and the moment before the window opens all look at the
// executable instead, and an executable with no icon resource is shown with
// Windows' blank placeholder. Ebitengine's GLFW port does not read a GLFW_ICON
// resource either, so nothing would fill that gap.
//
// The executable's icon is gumpet's own: a red mailbox with a letter, drawn in
// mailbox.go. The PNGs here are generated from that drawing, and the .syso
// files beside cmd/gumpet's main.go are built from the PNGs; regenerate both
// with
//
//	go generate ./cmd/gumpet
//
// The test in this package fails if the PNGs no longer match the drawing.
package winres
