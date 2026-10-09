// Package web holds the browser files, embedded into the server binary.
package web

import "embed"

// Files contains the HTML pages, css/ and js/.
//
//go:embed *.html css js
var Files embed.FS
