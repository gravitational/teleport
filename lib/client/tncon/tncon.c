// +build windows,cgo

/*
* Author: Ray Hayes <ray.hayes@microsoft.com>
* ANSI TTY Reader - Maps Windows console input events to ANSI stream
*
* Author: Balu <bagajjal@microsoft.com>
* Misc fixes and code cleanup
*
* Copyright (c) 2017 Microsoft Corp.
* All rights reserved
*
* Modifications for use in Teleport Copyright 2021 Gravitational, Inc.
*
* This file is responsible for console reading calls for building an emulator
* over Windows Console.
*
* Redistribution and use in source and binary forms, with or without
* modification, are permitted provided that the following conditions
* are met:
*
* 1. Redistributions of source code must retain the above copyright
* notice, this list of conditions and the following disclaimer.
* 2. Redistributions in binary form must reproduce the above copyright
* notice, this list of conditions and the following disclaimer in the
* documentation and/or other materials provided with the distribution.
*
* THIS SOFTWARE IS PROVIDED BY THE AUTHOR ``AS IS'' AND ANY EXPRESS OR
* IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES
* OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED.
* IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY DIRECT, INDIRECT,
* INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT
* NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
* DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
* THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
* (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF
* THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/
#include <stdlib.h>
#include <stdio.h>
#include <stddef.h>
#include <windows.h>
#include "ansiprsr.h"
#include "tncon.h"

#include "_cgo_export.h"

// NOTE: Derived from https://github.com/PowerShell/openssh-portable/blob/0b73c4636d38f0c69424721d52d0e7752db99c81/contrib/win32/win32compat/tncon.c

// WriteToBuffer is a dummy function that writes a sequence event rather than
// to a buffer.
void
WriteToBuffer(char* source, size_t len)
{
	// NOTE: Modified to emit an event to the Go lib rather than mutate a
	// global buffer.
	writeSequence(source, len);
}

// DataAvailable waits for either a new input event, a quit event, or for a
// handle to close.
BOOL
DataAvailable(HANDLE hInput, HANDLE hQuitEvent)
{
	const HANDLE handles[] = { hInput, hQuitEvent };
	DWORD dwRet = WaitForMultipleObjectsEx(2, handles, FALSE, INFINITE, TRUE);
	if (dwRet == WAIT_OBJECT_0) {
		// Data is ready.
		return TRUE;
	} else if (dwRet == WAIT_OBJECT_0 + 1) {
		// Quit signal is ready.
		return FALSE;
	} else if (dwRet == WAIT_FAILED) {
		return FALSE;
	}
	return FALSE;
}

void
ReadConsoleForTermEmul(HANDLE hInput, HANDLE hQuitEvent)
{
	DWORD dwInput = 0;
	unsigned char octets[20];
	INPUT_RECORD inputRecordArray[16];
	size_t inputRecordArraySize = sizeof(inputRecordArray) / sizeof(INPUT_RECORD);
	static WCHAR utf16_surrogatepair[2] = {0,};
	size_t n = 0;

	while (DataAvailable(hInput, hQuitEvent)) {
		ReadConsoleInputW(hInput, inputRecordArray, inputRecordArraySize, &dwInput);

		for (DWORD i=0; i < dwInput; i++) {
			INPUT_RECORD inputRecord = inputRecordArray[i];

			switch (inputRecord.EventType) {

			// NOTE: modified here to emit events directly
			case WINDOW_BUFFER_SIZE_EVENT:
				notifyResizeEvent();
				break;
			case FOCUS_EVENT:
				break;
			case MOUSE_EVENT:
				break;
			case MENU_EVENT:
				break;

			case KEY_EVENT:
				if ((inputRecord.Event.KeyEvent.bKeyDown) ||
				    (!inputRecord.Event.KeyEvent.bKeyDown && inputRecord.Event.KeyEvent.wVirtualKeyCode == VK_MENU)) {
					if (IS_HIGH_SURROGATE(inputRecord.Event.KeyEvent.uChar.UnicodeChar)) {
						utf16_surrogatepair[0] = inputRecord.Event.KeyEvent.uChar.UnicodeChar;
						break; // break to read low surrogate.
					}
					else if (IS_LOW_SURROGATE(inputRecord.Event.KeyEvent.uChar.UnicodeChar)) {
						utf16_surrogatepair[1] = inputRecord.Event.KeyEvent.uChar.UnicodeChar;
					}

					if (utf16_surrogatepair[0] && utf16_surrogatepair[1]) {
						n = WideCharToMultiByte(
							CP_UTF8,
							0,
							utf16_surrogatepair,
							2,
							(LPSTR)octets,
							20,
							NULL,
							NULL);

						WriteToBuffer((char *)octets, n);
						utf16_surrogatepair[0] = utf16_surrogatepair[1] = L'\0';

						break;
					}

					if (inputRecord.Event.KeyEvent.uChar.UnicodeChar != L'\0') {
						n = WideCharToMultiByte(
							CP_UTF8,
							0,
							&(inputRecord.Event.KeyEvent.uChar.UnicodeChar),
							1,
							(LPSTR)octets,
							20,
							NULL,
							NULL);

						WriteToBuffer((char *)octets, n);
					}
				}
				break;
			}
		}
	}
}

// ReadInputContinuous reads all console input events until the program exits,
// stdin closes, or the quit event is triggered.
void ReadInputContinuous(HANDLE hQuitEvent) {
	HANDLE hInput = GetStdHandle(STD_INPUT_HANDLE);
	ReadConsoleForTermEmul(hInput, hQuitEvent);
}
