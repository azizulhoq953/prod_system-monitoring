//go:build darwin

package main

import (
    "bytes"
    "os/exec"
    "strings"
)

func getActiveWindowTitle() string {
    script := `tell application "System Events" to get name of window 1 of (first application process whose frontmost is true)`
    
    cmd := exec.Command("osascript", "-e", script)
    var out bytes.Buffer
    cmd.Stdout = &out
    err := cmd.Run()
    if err != nil { 
        return "Unknown" 
    }
    return strings.TrimSpace(out.String())
}