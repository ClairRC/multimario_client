//go:build windows

package main

//This file adds specific logic for disabling windows QuickEdit mode because it messes with SSE updates

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func disableQuickEdit() {
    fd := os.Stdin.Fd()
    handle := windows.Handle(fd)

    var mode uint32
    err := windows.GetConsoleMode(handle, &mode)
    if err != nil {
        fmt.Printf("Error getting console mode: %v\n", err)
        return
    }

    mode &^= windows.ENABLE_QUICK_EDIT_MODE
    mode |= windows.ENABLE_EXTENDED_FLAGS

    err = windows.SetConsoleMode(handle, mode)
    if err != nil {
        fmt.Printf("Error setting console mode: %v\n", err)
        return
    }

	fmt.Println("Quick Edit mode has been disabled.")
}