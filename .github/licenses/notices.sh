#!/bin/sh
# Writes the third-party notices for one platform's build of gumpet to stdout:
# every Go module linked into it with its licence text, Go's own licence (the
# runtime is linked in too), the fonts compiled in with the bitmap font, and
# the bundled artwork.
#
#   .github/licenses/notices.sh linux amd64 > THIRD_PARTY_NOTICES
#
# Run from the repository root, with go-licenses on PATH. It is per platform
# because the modules are: xgb is linked on Linux only.
set -eu

goos=$1
goarch=$2
here=$(dirname "$0")

cat <<HEADER
gumpet includes software and artwork by others. Their licences require that
their notices travel with it; they are reproduced below. gumpet's own licence
is in LICENSE.

This file lists what is in the ${goos}/${goarch} build.

HEADER

# GOROOT is passed explicitly: go-licenses otherwise looks for the standard
# library where it was itself built, and when that is not the toolchain in use
# it reports every standard package as a module with no licence and fails.
GOROOT=$(go env GOROOT) GOOS=$goos GOARCH=$goarch \
	go-licenses report ./cmd/... \
	--ignore github.com/kaakaa/gumpet \
	--template "$here/notices.tpl"

rule="=============================================================================="

# Apache-2.0 also asks for a module's NOTICE file, which go-licenses does not
# report.
GOOS=$goos GOARCH=$goarch \
	go list -deps -f '{{with .Module}}{{.Path}} {{.Dir}}{{end}}' ./cmd/... |
	sort -u | while read -r path dir; do
		[ "$path" = github.com/kaakaa/gumpet ] && continue
		for name in NOTICE NOTICE.txt NOTICE.md; do
			if [ -f "$dir/$name" ]; then
				printf '\n%s\nNOTICE from %s\n%s\n\n' "$rule" "$path" "$rule"
				cat "$dir/$name"
			fi
		done
	done

printf '\n%s\nGo %s — BSD-3-Clause\nhttps://go.dev/LICENSE\n%s\n\n' "$rule" "$(go env GOVERSION)" "$rule"
cat "$(go env GOROOT)/LICENSE"
echo

cat "$here/fonts.txt"

for notice in assets/*/NOTICE; do
	pet=$(basename "$(dirname "$notice")")
	printf '\n%s\nArtwork: the %s pet\n%s\n\n' "$rule" "$pet" "$rule"
	cat "$notice"
done
