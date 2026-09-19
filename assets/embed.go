// Package assets holds the images gumpet ships with.
//
// Every directory carries a NOTICE naming where its artwork came from and
// under what terms. Two things are always separate there: the licence on the
// drawing, and the licence on the gopher itself, which is Renée French's
// regardless of who drew this particular one.
package assets

import "embed"

// Pets is every bundled pet. The NOTICE files are embedded alongside the
// images so that a binary carries its own attribution rather than relying on
// the repository being at hand.
//
//go:embed gopher/* pixel/* blue/* strawhat/* pink/*
var Pets embed.FS

// Gopher is the original bundled pet, kept as its own name because it is the
// default and several things reach for it directly.
//
//go:embed gopher/*.png
var Gopher embed.FS
