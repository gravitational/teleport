/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package tncon

import "unsafe"

//go:generate go run golang.org/x/sys/windows/mkwinsyscall -output zsyscall_windows.go syscall_windows.go
//sys	readConsoleInput(console windows.Handle, buf *inputRecord, toread uint32, read *uint32) (err error) = kernel32.ReadConsoleInputW

// inputRecord is INPUT_RECORD.
// https://learn.microsoft.com/en-us/windows/console/input-record-str
//
// The Event field is a union. KEY_EVENT_RECORD is its largest member, and the
// only one that is read.
type inputRecord struct {
	EventType uint16
	KeyEvent  keyEventRecord
}

// keyEventRecord is KEY_EVENT_RECORD.
// https://learn.microsoft.com/en-us/windows/console/key-event-record-str
type keyEventRecord struct {
	KeyDown         int32
	RepeatCount     uint16
	VirtualKeyCode  uint16
	VirtualScanCode uint16
	UnicodeChar     uint16
	ControlKeyState uint32
}

// The compile time assertions below verify that [inputRecord] has the same memory layout
// as the Win32 INPUT_RECORD struct: 20 bytes, with the union starting at offset 4.
var (
	_ [20 - unsafe.Sizeof(inputRecord{})]struct{}
	_ [unsafe.Sizeof(inputRecord{}) - 20]struct{}
	_ [4 - unsafe.Offsetof(inputRecord{}.KeyEvent)]struct{}
	_ [unsafe.Offsetof(inputRecord{}.KeyEvent) - 4]struct{}
)
