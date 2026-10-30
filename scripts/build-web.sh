#!/bin/sh
# Builds the playground for browsers into dist/web: a loader page with a
# spinner, Go's wasm_exec.js, and the app as WebAssembly. Serve that
# directory over HTTP; opening index.html from disk does not work, as
# browsers refuse to fetch the WebAssembly over file://.
#
# The page is what GitHub Actions publishes to GitHub Pages.
set -eu
cd "$(dirname "$0")/.."
go run github.com/egoist/mygo/cmd/mygo build -platform web
