//go:build linux

package main

import (
    "bytes"
    "os/exec"
    "strings"
)

func getActiveWindowTitle() string {
    cmd := exec.Command("xdotool", "getwindowfocus", "getwindowname")
    var out bytes.Buffer
    cmd.Stdout = &out
    err := cmd.Run()
    if err != nil { return "Unknown" }
    return strings.TrimSpace(out.String())
}