//go:build ui
// +build ui

package main

import "embed"

//go:embed hub-ui/build
var uiFiles embed.FS
