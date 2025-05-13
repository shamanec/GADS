//go:build !ui
// +build !ui

package main

import (
	"io/fs"
)

var uiFiles fs.FS = nil
