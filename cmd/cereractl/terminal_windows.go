//go:build windows

package main

import (
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

const terminalNeedsEnter = false

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	getStdHandle      = kernel32.NewProc("GetStdHandle")
	readConsoleInputW = kernel32.NewProc("ReadConsoleInputW")
	getConsoleMode    = kernel32.NewProc("GetConsoleMode")
	setConsoleMode    = kernel32.NewProc("SetConsoleMode")
	readConsoleW      = kernel32.NewProc("ReadConsoleW")
)

type consoleInputRecord struct {
	eventType uint16
	padding   uint16
	event     [16]byte
}

func readKey() (rune, error) {
	handle, _, callErr := getStdHandle.Call(uintptr(uint32(0xfffffff6))) // STD_INPUT_HANDLE
	if handle == 0 || handle == uintptr(syscall.InvalidHandle) {
		return 0, callErr
	}

	for {
		var record consoleInputRecord
		var count uint32
		ok, _, err := readConsoleInputW.Call(handle, uintptr(unsafe.Pointer(&record)), 1, uintptr(unsafe.Pointer(&count)))
		if ok == 0 {
			return 0, err
		}
		// KEY_EVENT and key-down; the character is at offset 10 in KEY_EVENT_RECORD.
		if record.eventType != 0x0001 || *(*int32)(unsafe.Pointer(&record.event[0])) == 0 {
			continue
		}
		char := *(*uint16)(unsafe.Pointer(&record.event[10]))
		if char != 0 {
			return rune(char), nil
		}
		return 0, errUnsupportedKey
	}
}

func setupTerminal() func() {
	handle, _, _ := getStdHandle.Call(uintptr(uint32(0xfffffff5))) // STD_OUTPUT_HANDLE
	if handle == 0 || handle == uintptr(syscall.InvalidHandle) {
		return func() {}
	}
	var mode uint32
	if ok, _, _ := getConsoleMode.Call(handle, uintptr(unsafe.Pointer(&mode))); ok == 0 {
		return func() {}
	}
	if ok, _, _ := setConsoleMode.Call(handle, uintptr(mode|0x0004)); ok == 0 { // ENABLE_VIRTUAL_TERMINAL_PROCESSING
		return func() {}
	}
	return func() { setConsoleMode.Call(handle, uintptr(mode)) }
}

func readTextLine() (string, error) {
	handle, _, callErr := getStdHandle.Call(uintptr(uint32(0xfffffff6))) // STD_INPUT_HANDLE
	if handle == 0 || handle == uintptr(syscall.InvalidHandle) {
		return "", callErr
	}
	buffer := make([]uint16, 4096)
	var read uint32
	ok, _, err := readConsoleW.Call(handle, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), uintptr(unsafe.Pointer(&read)), 0)
	if ok == 0 {
		return "", err
	}
	return strings.TrimRight(string(utf16.Decode(buffer[:read])), "\r\n"), nil
}
