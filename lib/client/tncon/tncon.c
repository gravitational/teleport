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
// Functions are largely kept unmodified, except where noted.

// NOTE: vars that are normally set dynamically.
//  - gbVTAppMode: some cursor functionality?
//  - isAnsiParsingRequired: only needed if term doesn't support VT escapes
//  - isConsoleVTSeqAvailable: if ENABLE_VIRTUAL_TERMINAL_INPUT is supported
// extern bool gbVTAppMode;
// extern BOOL isAnsiParsingRequired;
// extern BOOL isConsoleVTSeqAvailable;

// NOTE: Hardcoded defaults for the above vars. We ensure VT and ConPTY exist
// before deferring to this C code. Hard-coding `isConsoleVTSeqAvailable`
// implies the ENABLE_VIRTUAL_TERMINAL_INPUT flag is set; if we ever wish to
// support older Windows releases, the flag can be switched to enable VT
// sequence parsing.
const bool gbVTAppMode = false;
const BOOL isAnsiParsingRequired = FALSE;
const BOOL isConsoleVTSeqAvailable = TRUE;

char *glob_out = NULL;
int glob_outlen = 0;
int glob_space = 0;
unsigned char  NAWSSTR[] = { "\xff\xfa\x1f\x00\x00\x00\x00\xff\xf0" };
unsigned char tmp_buf[30];

/* terminal global switches*/
TelParams Parameters = {
	0,		/* int fLogging */
	NULL,		/* FILE *fplogfile */
	NULL,		/* char *pInputFile */
	NULL,		/* char *szDebugInputFile */
	FALSE,		/* BOOL fDebugWait */
	0,		/* int timeOut */
	0,		/* int fLocalEcho */
	0,		/* int fTreatLFasCRLF */
	0,		/* int	fSendCROnly */
	ENUM_LF,	/* int nReceiveCRLF */
	'`',		/* char sleepChar */
	'\035',		/* char menuChar; // CTRL-]  */
	0,		/* SOCKET Socket */
	FALSE,		/* BOOL bVT100Mode */
	"\x01",		/* char *pAltKey */
};

TelParams* pParams = &Parameters;

void GetVTSeqFromKeyStroke(INPUT_RECORD inputRecord);

/* Write to a global buffer setup by ReadConsoleForTermEmul() */
int
WriteToBuffer(char* source, size_t len)
{
	// NOTE: small modification to send off an event here.
	writeSequenceEvent(source, len);

	while (len > 0) {
		if (glob_outlen >= glob_space)
			return glob_outlen;
		*glob_out++ = *source++;
		len--;
		glob_outlen++;
	}
	return glob_outlen;
}

BOOL
DataAvailable(HANDLE h)
{
	DWORD dwRet = WaitForSingleObjectEx(h, INFINITE, TRUE);
	if (dwRet == WAIT_OBJECT_0)
		return TRUE;
	if (dwRet == WAIT_FAILED)
		return FALSE;
	return FALSE;
}

int
GetModifierKey(DWORD dwControlKeyState)
{
	int modKey = 0;
	if ((dwControlKeyState & LEFT_ALT_PRESSED) || (dwControlKeyState & RIGHT_ALT_PRESSED))
		modKey += 2;

	if (dwControlKeyState & SHIFT_PRESSED)
		modKey += 1;

	if ((dwControlKeyState & LEFT_CTRL_PRESSED) || (dwControlKeyState & RIGHT_CTRL_PRESSED))
		modKey += 4;

	if (modKey){
		memset(tmp_buf, 0, sizeof(tmp_buf));
		modKey++;
	}

	return modKey;
}

int
ReadConsoleForTermEmul(HANDLE hInput, char *destin, int destinlen)
{
	HANDLE hHandle[] = { hInput, NULL };
	DWORD nHandle = 1;
	DWORD dwInput = 0;
	DWORD rc = 0;
	unsigned char octets[20];
	char aChar = 0;
	INPUT_RECORD inputRecordArray[16];
	int inputRecordArraySize = sizeof(inputRecordArray) / sizeof(INPUT_RECORD);
	static WCHAR utf16_surrogatepair[2] = {0,};
	int n = 0;

	glob_out = destin;
	glob_space = destinlen;
	glob_outlen = 0;
	while (DataAvailable(hInput)) {
		if (glob_outlen >= destinlen)
			return glob_outlen;
		ReadConsoleInputW(hInput, inputRecordArray, inputRecordArraySize, &dwInput);

		for (DWORD i=0; i < dwInput; i++) {
			INPUT_RECORD inputRecord = inputRecordArray[i];

			switch (inputRecord.EventType) {

			// NOTE: modified here to emit events directly
			case WINDOW_BUFFER_SIZE_EVENT:
				writeResizeEvent(inputRecord.Event.WindowBufferSizeEvent.dwSize);
				break;
			case FOCUS_EVENT:
				writeFocusEvent(inputRecord.Event.FocusEvent.bSetFocus);
				break;
			case MOUSE_EVENT:
				writeMouseEvent();
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

					if (isConsoleVTSeqAvailable) {
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
					} else {
						GetVTSeqFromKeyStroke(inputRecord);
					}
				}
				break;
			}
		}
		break;
	}

	return glob_outlen;
}

void
GetVTSeqFromKeyStroke(INPUT_RECORD inputRecord)
{
	unsigned char octets[20];
	BOOL bCapsOn = FALSE;
	BOOL bShift = FALSE;
	int modKey = 0;
	char *FN_KEY = NULL;
	char *SHIFT_FN_KEY = NULL;
	char *ALT_FN_KEY = NULL;
	char *CTRL_FN_KEY = NULL;
	char *SHIFT_ALT_FN_KEY = NULL;
	char *SHIFT_CTRL_FN_KEY = NULL;
	char *ALT_CTRL_FN_KEY = NULL;
	char *SHIFT_ALT_CTRL_FN_KEY = NULL;
	DWORD dwControlKeyState = 0;
	DWORD dwAltGrFlags = LEFT_CTRL_PRESSED | RIGHT_ALT_PRESSED;

	int n = WideCharToMultiByte(
		CP_UTF8,
		0,
		&(inputRecord.Event.KeyEvent.uChar.UnicodeChar),
		1,
		(LPSTR)octets,
		20,
		NULL,
		NULL);

	bCapsOn = (inputRecord.Event.KeyEvent.dwControlKeyState & CAPSLOCK_ON);
	bShift = (inputRecord.Event.KeyEvent.dwControlKeyState & SHIFT_PRESSED);
	dwControlKeyState = inputRecord.Event.KeyEvent.dwControlKeyState &
		~(CAPSLOCK_ON | ENHANCED_KEY | NUMLOCK_ON | SCROLLLOCK_ON);

	/* ignore the AltGr flags*/
	if ((dwControlKeyState & dwAltGrFlags) == dwAltGrFlags)
		dwControlKeyState = dwControlKeyState & ~dwAltGrFlags;

	modKey = GetModifierKey(dwControlKeyState);

    // NOTE: This function is otherwise left unmodified, however including
    // ConWriteString would introduce a large additional tree of dependencies.
    // We don't need local echo, so we'll just disable this.
	// if (pParams->fLocalEcho)
	// 	ConWriteString((char *)octets, n);

	switch (inputRecord.Event.KeyEvent.uChar.UnicodeChar) {
	case 0xd:
		if (pParams->nReceiveCRLF == ENUM_LF)
			WriteToBuffer("\r", 1);
		else
			WriteToBuffer("\r\n", 2);
		break;

	case VK_ESCAPE:
		WriteToBuffer((char *)ESCAPE_KEY, 1);
		break;

	default:
		switch (inputRecord.Event.KeyEvent.wVirtualKeyCode) {
		case VK_UP:
			if (!modKey)
				WriteToBuffer((char *)(gbVTAppMode ? APP_UP_ARROW : UP_ARROW), 3);
			else {
				/* ^[[1;mA */
				char *p = "\033[1;";
				strcpy_s(tmp_buf, sizeof(tmp_buf), p);
				size_t index = strlen(p);
				tmp_buf[index++] = modKey + '0';
				tmp_buf[index] = 'A';

				WriteToBuffer(tmp_buf, index + 1);
			}
			break;
		case VK_DOWN:
			if (!modKey)
				WriteToBuffer((char *)(gbVTAppMode ? APP_DOWN_ARROW : DOWN_ARROW), 3);
			else {
				/* ^[[1;mB */
				char *p = "\033[1;";
				strcpy_s(tmp_buf, sizeof(tmp_buf), p);
				size_t index = strlen(p);
				tmp_buf[index++] = modKey + '0';
				tmp_buf[index] = 'B';

				WriteToBuffer(tmp_buf, index + 1);
			}
			break;
		case VK_RIGHT:
			if (!modKey)
				WriteToBuffer((char *)(gbVTAppMode ? APP_RIGHT_ARROW : RIGHT_ARROW), 3);
			else {
				/* ^[[1;mC */
				char *p = "\033[1;";
				strcpy_s(tmp_buf, sizeof(tmp_buf), p);
				size_t index = strlen(p);
				tmp_buf[index++] = modKey + '0';
				tmp_buf[index] = 'C';

				WriteToBuffer(tmp_buf, index + 1);
			}
			break;
		case VK_LEFT:
			if (!modKey)
				WriteToBuffer((char *)(gbVTAppMode ? APP_LEFT_ARROW : LEFT_ARROW), 3);
			else {
				/* ^[[1;mD */
				char *p = "\033[1;";
				strcpy_s(tmp_buf, sizeof(tmp_buf), p);
				size_t index = strlen(p);
				tmp_buf[index++] = modKey + '0';
				tmp_buf[index] = 'D';

				WriteToBuffer(tmp_buf, index + 1);
			}
			break;
		case VK_END:
			if (!modKey)
				WriteToBuffer((char *)SELECT_KEY, 4);
			else {
				/* ^[[1;mF */
				char *p = "\033[1;";
				strcpy_s(tmp_buf, sizeof(tmp_buf), p);
				size_t index = strlen(p);
				tmp_buf[index++] = modKey + '0';
				tmp_buf[index] = 'F';

				WriteToBuffer(tmp_buf, index + 1);
			}
			break;
		case VK_HOME:
			if (!modKey)
				WriteToBuffer((char *)FIND_KEY, 4);
			else {
				/* ^[[1;mH */
				char *p = "\033[1;";
				strcpy_s(tmp_buf, sizeof(tmp_buf), p);
				size_t index = strlen(p);
				tmp_buf[index++] = modKey + '0';
				tmp_buf[index] = 'H';

				WriteToBuffer(tmp_buf, index + 1);
			}
			break;
		case VK_INSERT:
			if (!modKey)
				WriteToBuffer((char *)INSERT_KEY, 4);
			else {
				/* ^[[2;m~ */
				char *p = "\033[2;";
				strcpy_s(tmp_buf, sizeof(tmp_buf), p);
				size_t index = strlen(p);
				tmp_buf[index++] = modKey + '0';
				tmp_buf[index] = '~';

				WriteToBuffer(tmp_buf, index + 1);
			}
			break;
		case VK_DELETE:
			if (!modKey)
				WriteToBuffer((char *)REMOVE_KEY, 4);
			else {
				/* ^[[3;m~ */
				char *p = "\033[3;";
				strcpy_s(tmp_buf, sizeof(tmp_buf), p);
				size_t index = strlen(p);
				tmp_buf[index++] = modKey + '0';
				tmp_buf[index] = '~';

				WriteToBuffer(tmp_buf, index + 1);
			}
			break;
		case VK_PRIOR: /* page up */
			if (!modKey)
				WriteToBuffer((char *)PREV_KEY, 4);
			else {
				/* ^[[5;m~ */
				char *p = "\033[5;";
				strcpy_s(tmp_buf, sizeof(tmp_buf), p);
				size_t index = strlen(p);
				tmp_buf[index++] = modKey + '0';
				tmp_buf[index] = '~';

				WriteToBuffer(tmp_buf, index + 1);
			}
			break;
		case VK_NEXT: /* page down */
			if (!modKey)
				WriteToBuffer((char *)NEXT_KEY, 4);
			else {
				/* ^[[6;m~  */
				char *p = "\033[6;";
				strcpy_s(tmp_buf, sizeof(tmp_buf), p);
				size_t index = strlen(p);
				tmp_buf[index++] = modKey + '0';
				tmp_buf[index] = '~';

				WriteToBuffer(tmp_buf, index + 1);
			}
			break;
		case VK_BACK:
			WriteToBuffer((char *)BACKSPACE_KEY, 1);
			break;
		case VK_TAB:
			if (dwControlKeyState == SHIFT_PRESSED)
				WriteToBuffer((char *)SHIFT_TAB_KEY, 3);
			else
				WriteToBuffer((char *)octets, n);
			break;
		case VK_ESCAPE:
			WriteToBuffer((char *)ESCAPE_KEY, 1);
			break;
		case VK_SHIFT:
		case VK_CONTROL:
		case VK_CAPITAL:
			break; /* NOP on these */
		case VK_F1:
			/* If isAnsiParsingRequired is false then we use XTERM VT sequence */
			FN_KEY = isAnsiParsingRequired ? PF1_KEY : XTERM_PF1_KEY;
			SHIFT_FN_KEY = isAnsiParsingRequired ? SHIFT_PF1_KEY : XTERM_SHIFT_PF1_KEY;
			ALT_FN_KEY = isAnsiParsingRequired ? ALT_PF1_KEY : XTERM_ALT_PF1_KEY;
			CTRL_FN_KEY = isAnsiParsingRequired ? CTRL_PF1_KEY : XTERM_CTRL_PF1_KEY;
			SHIFT_ALT_FN_KEY = isAnsiParsingRequired ? SHIFT_ALT_PF1_KEY : XTERM_SHIFT_ALT_PF1_KEY;
			SHIFT_CTRL_FN_KEY = isAnsiParsingRequired ? SHIFT_CTRL_PF1_KEY : XTERM_SHIFT_CTRL_PF1_KEY;
			ALT_CTRL_FN_KEY = isAnsiParsingRequired ? ALT_CTRL_PF1_KEY : XTERM_ALT_CTRL_PF1_KEY;
			SHIFT_ALT_CTRL_FN_KEY = isAnsiParsingRequired ? SHIFT_ALT_CTRL_PF1_KEY : XTERM_SHIFT_ALT_CTRL_PF1_KEY;

			if (dwControlKeyState == 0)
				WriteToBuffer((char *)FN_KEY, strlen(FN_KEY));

			else if (dwControlKeyState == SHIFT_PRESSED)
				WriteToBuffer((char *)SHIFT_FN_KEY, strlen(SHIFT_FN_KEY));

			else if (dwControlKeyState == LEFT_CTRL_PRESSED || dwControlKeyState == RIGHT_CTRL_PRESSED)
				WriteToBuffer((char *)CTRL_FN_KEY, strlen(CTRL_FN_KEY));

			else if (dwControlKeyState == LEFT_ALT_PRESSED || dwControlKeyState == RIGHT_ALT_PRESSED)
				WriteToBuffer((char *)ALT_FN_KEY, strlen(ALT_FN_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_CTRL_FN_KEY, strlen(SHIFT_ALT_CTRL_FN_KEY));

			else if ((dwControlKeyState & RIGHT_ALT_PRESSED) || (dwControlKeyState & LEFT_ALT_PRESSED) &&
				((dwControlKeyState & LEFT_CTRL_PRESSED) || (dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)ALT_CTRL_FN_KEY, strlen(ALT_CTRL_FN_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_FN_KEY, strlen(SHIFT_ALT_FN_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_CTRL_FN_KEY, strlen(SHIFT_CTRL_FN_KEY));

			break;
		case VK_F2:
			/* If isAnsiParsingRequired is false then we use XTERM VT sequence */
			FN_KEY = isAnsiParsingRequired ? PF2_KEY : XTERM_PF2_KEY;
			SHIFT_FN_KEY = isAnsiParsingRequired ? SHIFT_PF2_KEY : XTERM_SHIFT_PF2_KEY;
			ALT_FN_KEY = isAnsiParsingRequired ? ALT_PF2_KEY : XTERM_ALT_PF2_KEY;
			CTRL_FN_KEY = isAnsiParsingRequired ? CTRL_PF2_KEY : XTERM_CTRL_PF2_KEY;
			SHIFT_ALT_FN_KEY = isAnsiParsingRequired ? SHIFT_ALT_PF2_KEY : XTERM_SHIFT_ALT_PF2_KEY;
			SHIFT_CTRL_FN_KEY = isAnsiParsingRequired ? SHIFT_CTRL_PF2_KEY : XTERM_SHIFT_CTRL_PF2_KEY;
			ALT_CTRL_FN_KEY = isAnsiParsingRequired ? ALT_CTRL_PF2_KEY : XTERM_ALT_CTRL_PF2_KEY;
			SHIFT_ALT_CTRL_FN_KEY = isAnsiParsingRequired ? SHIFT_ALT_CTRL_PF2_KEY : XTERM_SHIFT_ALT_CTRL_PF2_KEY;

			if (dwControlKeyState == 0)
				WriteToBuffer((char *)FN_KEY, strlen(FN_KEY));

			else if (dwControlKeyState == SHIFT_PRESSED)
				WriteToBuffer((char *)SHIFT_FN_KEY, strlen(SHIFT_FN_KEY));

			else if (dwControlKeyState == LEFT_CTRL_PRESSED || dwControlKeyState == RIGHT_CTRL_PRESSED)
				WriteToBuffer((char *)CTRL_FN_KEY, strlen(CTRL_FN_KEY));

			else if (dwControlKeyState == LEFT_ALT_PRESSED || dwControlKeyState == RIGHT_ALT_PRESSED)
				WriteToBuffer((char *)ALT_FN_KEY, strlen(ALT_FN_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_CTRL_FN_KEY, strlen(SHIFT_ALT_CTRL_FN_KEY));

			else if ((dwControlKeyState & RIGHT_ALT_PRESSED) || (dwControlKeyState & LEFT_ALT_PRESSED) &&
				((dwControlKeyState & LEFT_CTRL_PRESSED) || (dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)ALT_CTRL_FN_KEY, strlen(ALT_CTRL_FN_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_FN_KEY, strlen(SHIFT_ALT_FN_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_CTRL_FN_KEY, strlen(SHIFT_CTRL_FN_KEY));

			break;
		case VK_F3:
			/* If isAnsiParsingRequired is false then we use XTERM VT sequence */
			FN_KEY = isAnsiParsingRequired ? PF3_KEY : XTERM_PF3_KEY;
			SHIFT_FN_KEY = isAnsiParsingRequired ? SHIFT_PF3_KEY : XTERM_SHIFT_PF3_KEY;
			ALT_FN_KEY = isAnsiParsingRequired ? ALT_PF3_KEY : XTERM_ALT_PF3_KEY;
			CTRL_FN_KEY = isAnsiParsingRequired ? CTRL_PF3_KEY : XTERM_CTRL_PF3_KEY;
			SHIFT_ALT_FN_KEY = isAnsiParsingRequired ? SHIFT_ALT_PF3_KEY : XTERM_SHIFT_ALT_PF3_KEY;
			SHIFT_CTRL_FN_KEY = isAnsiParsingRequired ? SHIFT_CTRL_PF3_KEY : XTERM_SHIFT_CTRL_PF3_KEY;
			ALT_CTRL_FN_KEY = isAnsiParsingRequired ? ALT_CTRL_PF3_KEY : XTERM_ALT_CTRL_PF3_KEY;
			SHIFT_ALT_CTRL_FN_KEY = isAnsiParsingRequired ? SHIFT_ALT_CTRL_PF3_KEY : XTERM_SHIFT_ALT_CTRL_PF3_KEY;

			if (dwControlKeyState == 0)
				WriteToBuffer((char *)FN_KEY, strlen(FN_KEY));

			else if (dwControlKeyState == SHIFT_PRESSED)
				WriteToBuffer((char *)SHIFT_FN_KEY, strlen(SHIFT_FN_KEY));

			else if (dwControlKeyState == LEFT_CTRL_PRESSED || dwControlKeyState == RIGHT_CTRL_PRESSED)
				WriteToBuffer((char *)CTRL_FN_KEY, strlen(CTRL_FN_KEY));

			else if (dwControlKeyState == LEFT_ALT_PRESSED || dwControlKeyState == RIGHT_ALT_PRESSED)
				WriteToBuffer((char *)ALT_FN_KEY, strlen(ALT_FN_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_CTRL_FN_KEY, strlen(SHIFT_ALT_CTRL_FN_KEY));

			else if ((dwControlKeyState & RIGHT_ALT_PRESSED) || (dwControlKeyState & LEFT_ALT_PRESSED) &&
				((dwControlKeyState & LEFT_CTRL_PRESSED) || (dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)ALT_CTRL_FN_KEY, strlen(ALT_CTRL_FN_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_FN_KEY, strlen(SHIFT_ALT_FN_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_CTRL_FN_KEY, strlen(SHIFT_CTRL_FN_KEY));

			break;
		case VK_F4:
			/* If isAnsiParsingRequired is false then we use XTERM VT sequence */
			FN_KEY = isAnsiParsingRequired ? PF4_KEY : XTERM_PF4_KEY;
			SHIFT_FN_KEY = isAnsiParsingRequired ? SHIFT_PF4_KEY : XTERM_SHIFT_PF4_KEY;
			ALT_FN_KEY = isAnsiParsingRequired ? ALT_PF4_KEY : XTERM_ALT_PF4_KEY;
			CTRL_FN_KEY = isAnsiParsingRequired ? CTRL_PF4_KEY : XTERM_CTRL_PF4_KEY;
			SHIFT_ALT_FN_KEY = isAnsiParsingRequired ? SHIFT_ALT_PF4_KEY : XTERM_SHIFT_ALT_PF4_KEY;
			SHIFT_CTRL_FN_KEY = isAnsiParsingRequired ? SHIFT_CTRL_PF4_KEY : XTERM_SHIFT_CTRL_PF4_KEY;
			ALT_CTRL_FN_KEY = isAnsiParsingRequired ? ALT_CTRL_PF4_KEY : XTERM_ALT_CTRL_PF4_KEY;
			SHIFT_ALT_CTRL_FN_KEY = isAnsiParsingRequired ? SHIFT_ALT_CTRL_PF4_KEY : XTERM_SHIFT_ALT_CTRL_PF4_KEY;

			if (dwControlKeyState == 0)
				WriteToBuffer((char *)FN_KEY, strlen(FN_KEY));

			else if (dwControlKeyState == SHIFT_PRESSED)
				WriteToBuffer((char *)SHIFT_FN_KEY, strlen(SHIFT_FN_KEY));

			else if (dwControlKeyState == LEFT_CTRL_PRESSED || dwControlKeyState == RIGHT_CTRL_PRESSED)
				WriteToBuffer((char *)CTRL_FN_KEY, strlen(CTRL_FN_KEY));

			else if (dwControlKeyState == LEFT_ALT_PRESSED || dwControlKeyState == RIGHT_ALT_PRESSED)
				WriteToBuffer((char *)ALT_FN_KEY, strlen(ALT_FN_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_CTRL_FN_KEY, strlen(SHIFT_ALT_CTRL_FN_KEY));

			else if ((dwControlKeyState & RIGHT_ALT_PRESSED) || (dwControlKeyState & LEFT_ALT_PRESSED) &&
				((dwControlKeyState & LEFT_CTRL_PRESSED) || (dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)ALT_CTRL_FN_KEY, strlen(ALT_CTRL_FN_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_FN_KEY, strlen(SHIFT_ALT_FN_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_CTRL_FN_KEY, strlen(SHIFT_CTRL_FN_KEY));

			break;
		case VK_F5:
			if (dwControlKeyState == 0)
				WriteToBuffer((char *)PF5_KEY, strlen(PF5_KEY));

			else if (dwControlKeyState == SHIFT_PRESSED)
				WriteToBuffer((char *)SHIFT_PF5_KEY, strlen(SHIFT_PF5_KEY));

			else if (dwControlKeyState == LEFT_CTRL_PRESSED || dwControlKeyState == RIGHT_CTRL_PRESSED)
				WriteToBuffer((char *)CTRL_PF5_KEY, strlen(CTRL_PF5_KEY));

			else if (dwControlKeyState == LEFT_ALT_PRESSED || dwControlKeyState == RIGHT_ALT_PRESSED)
				WriteToBuffer((char *)ALT_PF5_KEY, strlen(ALT_PF5_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_CTRL_PF5_KEY, strlen(SHIFT_ALT_CTRL_PF5_KEY));

			else if ((dwControlKeyState & RIGHT_ALT_PRESSED) || (dwControlKeyState & LEFT_ALT_PRESSED) &&
				((dwControlKeyState & LEFT_CTRL_PRESSED) || (dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)ALT_CTRL_PF5_KEY, strlen(ALT_CTRL_PF5_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_PF5_KEY, strlen(SHIFT_ALT_PF5_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_CTRL_PF5_KEY, strlen(SHIFT_CTRL_PF5_KEY));
			break;
		case VK_F6:
			if (dwControlKeyState == 0)
				WriteToBuffer((char *)PF6_KEY, strlen(PF6_KEY));

			else if (dwControlKeyState == SHIFT_PRESSED)
				WriteToBuffer((char *)SHIFT_PF6_KEY, strlen(SHIFT_PF6_KEY));

			else if (dwControlKeyState == LEFT_CTRL_PRESSED || dwControlKeyState == RIGHT_CTRL_PRESSED)
				WriteToBuffer((char *)CTRL_PF6_KEY, strlen(CTRL_PF6_KEY));

			else if (dwControlKeyState == LEFT_ALT_PRESSED || dwControlKeyState == RIGHT_ALT_PRESSED)
				WriteToBuffer((char *)ALT_PF6_KEY, strlen(ALT_PF6_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_CTRL_PF6_KEY, strlen(SHIFT_ALT_CTRL_PF6_KEY));

			else if ((dwControlKeyState & RIGHT_ALT_PRESSED) || (dwControlKeyState & LEFT_ALT_PRESSED) &&
				((dwControlKeyState & LEFT_CTRL_PRESSED) || (dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)ALT_CTRL_PF6_KEY, strlen(ALT_CTRL_PF6_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_PF6_KEY, strlen(SHIFT_ALT_PF6_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_CTRL_PF6_KEY, strlen(SHIFT_CTRL_PF6_KEY));
			break;
		case VK_F7:
			if (dwControlKeyState == 0)
				WriteToBuffer((char *)PF7_KEY, strlen(PF7_KEY));

			else if (dwControlKeyState == SHIFT_PRESSED)
				WriteToBuffer((char *)SHIFT_PF7_KEY, strlen(SHIFT_PF7_KEY));

			else if (dwControlKeyState == LEFT_CTRL_PRESSED || dwControlKeyState == RIGHT_CTRL_PRESSED)
				WriteToBuffer((char *)CTRL_PF7_KEY, strlen(CTRL_PF7_KEY));

			else if (dwControlKeyState == LEFT_ALT_PRESSED || dwControlKeyState == RIGHT_ALT_PRESSED)
				WriteToBuffer((char *)ALT_PF7_KEY, strlen(ALT_PF7_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_CTRL_PF7_KEY, strlen(SHIFT_ALT_CTRL_PF7_KEY));

			else if ((dwControlKeyState & RIGHT_ALT_PRESSED) || (dwControlKeyState & LEFT_ALT_PRESSED) &&
				((dwControlKeyState & LEFT_CTRL_PRESSED) || (dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)ALT_CTRL_PF7_KEY, strlen(ALT_CTRL_PF7_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_PF7_KEY, strlen(SHIFT_ALT_PF7_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_CTRL_PF7_KEY, strlen(SHIFT_CTRL_PF7_KEY));
			break;
		case VK_F8:
			if (dwControlKeyState == 0)
				WriteToBuffer((char *)PF8_KEY, strlen(PF8_KEY));

			else if (dwControlKeyState == SHIFT_PRESSED)
				WriteToBuffer((char *)SHIFT_PF8_KEY, strlen(SHIFT_PF8_KEY));

			else if (dwControlKeyState == LEFT_CTRL_PRESSED || dwControlKeyState == RIGHT_CTRL_PRESSED)
				WriteToBuffer((char *)CTRL_PF8_KEY, strlen(CTRL_PF8_KEY));

			else if (dwControlKeyState == LEFT_ALT_PRESSED || dwControlKeyState == RIGHT_ALT_PRESSED)
				WriteToBuffer((char *)ALT_PF8_KEY, strlen(ALT_PF8_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_CTRL_PF8_KEY, strlen(SHIFT_ALT_CTRL_PF8_KEY));

			else if ((dwControlKeyState & RIGHT_ALT_PRESSED) || (dwControlKeyState & LEFT_ALT_PRESSED) &&
				((dwControlKeyState & LEFT_CTRL_PRESSED) || (dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)ALT_CTRL_PF8_KEY, strlen(ALT_CTRL_PF8_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_PF8_KEY, strlen(SHIFT_ALT_PF8_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_CTRL_PF8_KEY, strlen(SHIFT_CTRL_PF8_KEY));
			break;
		case VK_F9:
			if (dwControlKeyState == 0)
				WriteToBuffer((char *)PF9_KEY, strlen(PF9_KEY));

			else if (dwControlKeyState == SHIFT_PRESSED)
				WriteToBuffer((char *)SHIFT_PF9_KEY, strlen(SHIFT_PF9_KEY));

			else if (dwControlKeyState == LEFT_CTRL_PRESSED || dwControlKeyState == RIGHT_CTRL_PRESSED)
				WriteToBuffer((char *)CTRL_PF9_KEY, strlen(CTRL_PF9_KEY));

			else if (dwControlKeyState == LEFT_ALT_PRESSED || dwControlKeyState == RIGHT_ALT_PRESSED)
				WriteToBuffer((char *)ALT_PF9_KEY, strlen(ALT_PF9_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_CTRL_PF9_KEY, strlen(SHIFT_ALT_CTRL_PF9_KEY));

			else if ((dwControlKeyState & RIGHT_ALT_PRESSED) || (dwControlKeyState & LEFT_ALT_PRESSED) &&
				((dwControlKeyState & LEFT_CTRL_PRESSED) || (dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)ALT_CTRL_PF9_KEY, strlen(ALT_CTRL_PF9_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_PF9_KEY, strlen(SHIFT_ALT_PF9_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_CTRL_PF9_KEY, strlen(SHIFT_CTRL_PF9_KEY));
			break;
		case VK_F10:
			if (dwControlKeyState == 0)
				WriteToBuffer((char *)PF10_KEY, strlen(PF10_KEY));

			else if (dwControlKeyState == SHIFT_PRESSED)
				WriteToBuffer((char *)SHIFT_PF10_KEY, strlen(SHIFT_PF10_KEY));

			else if (dwControlKeyState == LEFT_CTRL_PRESSED || dwControlKeyState == RIGHT_CTRL_PRESSED)
				WriteToBuffer((char *)CTRL_PF10_KEY, strlen(CTRL_PF10_KEY));

			else if (dwControlKeyState == LEFT_ALT_PRESSED || dwControlKeyState == RIGHT_ALT_PRESSED)
				WriteToBuffer((char *)ALT_PF10_KEY, strlen(ALT_PF10_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_CTRL_PF10_KEY, strlen(SHIFT_ALT_CTRL_PF10_KEY));

			else if ((dwControlKeyState & RIGHT_ALT_PRESSED) || (dwControlKeyState & LEFT_ALT_PRESSED) &&
				((dwControlKeyState & LEFT_CTRL_PRESSED) || (dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)ALT_CTRL_PF10_KEY, strlen(ALT_CTRL_PF10_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_PF10_KEY, strlen(SHIFT_ALT_PF10_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_CTRL_PF10_KEY, strlen(SHIFT_CTRL_PF10_KEY));
			break;
		case VK_F11:
			if (dwControlKeyState == 0)
				WriteToBuffer((char *)PF11_KEY, strlen(PF11_KEY));

			else if (dwControlKeyState == SHIFT_PRESSED)
				WriteToBuffer((char *)SHIFT_PF11_KEY, strlen(SHIFT_PF11_KEY));

			else if (dwControlKeyState == LEFT_CTRL_PRESSED || dwControlKeyState == RIGHT_CTRL_PRESSED)
				WriteToBuffer((char *)CTRL_PF11_KEY, strlen(CTRL_PF11_KEY));

			else if (dwControlKeyState == LEFT_ALT_PRESSED || dwControlKeyState == RIGHT_ALT_PRESSED)
				WriteToBuffer((char *)ALT_PF11_KEY, strlen(ALT_PF11_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_CTRL_PF11_KEY, strlen(SHIFT_ALT_CTRL_PF11_KEY));

			else if ((dwControlKeyState & RIGHT_ALT_PRESSED) || (dwControlKeyState & LEFT_ALT_PRESSED) &&
				((dwControlKeyState & LEFT_CTRL_PRESSED) || (dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)ALT_CTRL_PF11_KEY, strlen(ALT_CTRL_PF11_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_PF11_KEY, strlen(SHIFT_ALT_PF11_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_CTRL_PF11_KEY, strlen(SHIFT_CTRL_PF11_KEY));
			break;
		case VK_F12:
			if (dwControlKeyState == 0)
				WriteToBuffer((char *)PF12_KEY, strlen(PF12_KEY));

			else if (dwControlKeyState == SHIFT_PRESSED)
				WriteToBuffer((char *)SHIFT_PF12_KEY, strlen(SHIFT_PF12_KEY));

			else if (dwControlKeyState == LEFT_CTRL_PRESSED || dwControlKeyState == RIGHT_CTRL_PRESSED)
				WriteToBuffer((char *)CTRL_PF12_KEY, strlen(CTRL_PF12_KEY));

			else if (dwControlKeyState == LEFT_ALT_PRESSED || dwControlKeyState == RIGHT_ALT_PRESSED)
				WriteToBuffer((char *)ALT_PF12_KEY, strlen(ALT_PF12_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_CTRL_PF12_KEY, strlen(SHIFT_ALT_CTRL_PF12_KEY));

			else if ((dwControlKeyState & RIGHT_ALT_PRESSED) || (dwControlKeyState & LEFT_ALT_PRESSED) &&
				((dwControlKeyState & LEFT_CTRL_PRESSED) || (dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)ALT_CTRL_PF12_KEY, strlen(ALT_CTRL_PF12_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & RIGHT_ALT_PRESSED) ||
				(dwControlKeyState & LEFT_ALT_PRESSED)))
				WriteToBuffer((char *)SHIFT_ALT_PF12_KEY, strlen(SHIFT_ALT_PF12_KEY));

			else if ((dwControlKeyState & SHIFT_PRESSED) && ((dwControlKeyState & LEFT_CTRL_PRESSED) ||
				(dwControlKeyState & RIGHT_CTRL_PRESSED)))
				WriteToBuffer((char *)SHIFT_CTRL_PF12_KEY, strlen(SHIFT_CTRL_PF12_KEY));
			break;
		default:
			if (inputRecord.Event.KeyEvent.uChar.UnicodeChar != L'\0') {
				if ((dwControlKeyState & LEFT_ALT_PRESSED) || (dwControlKeyState & RIGHT_ALT_PRESSED)) {
					memset(tmp_buf, 0, sizeof(tmp_buf));
					tmp_buf[0] = '\x1b';
					memcpy(tmp_buf + 1, (char *)octets, n);
					WriteToBuffer(tmp_buf, n + 1);
				}
				else
					WriteToBuffer((char *)octets, n);
				break;
			}
		}
	}
}

// ReadInputContinuous reads all console input events until the program exits.
// This is a blocking call and will not return; callers should spawn this in
// a background thread and subscribe to events via the Go interface.
void ReadInputContinuous() {
	HANDLE hInput = GetStdHandle(STD_INPUT_HANDLE);
	char buf[32];

	while (true) {
		ReadConsoleForTermEmul(hInput, buf, 32);
	}
}
