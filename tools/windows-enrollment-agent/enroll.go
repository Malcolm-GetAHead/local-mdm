//go:build windows

package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	dll  = windows.NewLazySystemDLL("mdmregistration.dll")
	proc = dll.NewProc("RegisterDeviceWithManagement")
)

func main() {
	url := "https://192.168.1.229:8443/EnrollmentServer/Discovery.svc"
	if len(os.Args) > 1 {
		url = os.Args[1]
	}
	fmt.Println("URL:", url)

	upnPtr, _ := syscall.UTF16PtrFromString("")
	urlPtr, _ := syscall.UTF16PtrFromString(url)
	tokPtr, _ := syscall.UTF16PtrFromString("")

	dll.Load()
	proc.Find()

	code, _, err := proc.Call(
		uintptr(unsafe.Pointer(upnPtr)),
		uintptr(unsafe.Pointer(urlPtr)),
		uintptr(unsafe.Pointer(tokPtr)),
	)
	fmt.Printf("Result: %#x (%v)\n", code, err)
	if code != 0 {
		os.Exit(1)
	}
	fmt.Println("SUCCESS!")
}
