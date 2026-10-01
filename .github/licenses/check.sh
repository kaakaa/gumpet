#!/bin/sh
# Fails if anything linked into gumpet, on any platform it is released for, is
# under a licence not on the list below. Run from the repository root, with
# go-licenses on PATH.
#
# The list is what can be shipped inside an MIT-licensed binary by copying
# notices alone — which .github/licenses/notices.sh does. Anything else (the
# GPL family, the MPL, anything unrecognised) needs a decision rather than a
# line added here.
set -eu

allowed=Apache-2.0,BSD-2-Clause,BSD-3-Clause,MIT,ISC,Unlicense

for goos in darwin linux windows; do
	echo "checking $goos" >&2
	# See notices.sh for why GOROOT is spelled out.
	GOROOT=$(go env GOROOT) GOOS=$goos GOARCH=amd64 \
		go-licenses check ./cmd/... \
		--ignore github.com/kaakaa/gumpet \
		--allowed_licenses=$allowed
done
