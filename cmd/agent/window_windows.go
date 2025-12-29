//go:build windows

package main

import (
    "syscall"
    "unsafe"
)

var (
    user32                   = syscall.NewLazyDLL("user32.dll")
    procGetForegroundWindow  = user32.NewProc("GetForegroundWindow")
    procGetWindowTextW       = user32.NewProc("GetWindowTextW")
    procGetWindowTextLengthW = user32.NewProc("GetWindowTextLengthW")
)

func getActiveWindowTitle() string {
    hwnd, _, _ := procGetForegroundWindow.Call()
    if hwnd == 0 { return "" }
    len, _, _ := procGetWindowTextLengthW.Call(hwnd)
    if len == 0 { return "" }
    buf := make([]uint16, len+1)
    procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len+1))
    return syscall.UTF16ToString(buf)
}