// Package assets holds the images gumpet ships with.
package assets

import "embed"

// Gopher is the default pet: the walking gopher frames from mattn/gopher.
// The artwork is by Renée French (CC BY 3.0); see gopher/NOTICE.
//
//go:embed gopher/*.png
var Gopher embed.FS
