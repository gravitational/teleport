/*
 * Author: Microsoft Corp.
 *
 * Copyright (c) 2015 Microsoft Corp.
 * All rights reserved
 *
 * Microsoft openssh win32 port
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
/* console.h
 * 
 * Common library for Windows Console Screen IO.
 * Contains Windows console related definition so that emulation code can draw
 * on Windows console screen surface.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions are met:
 *
 */
 
#ifndef __PRAGMA_CONSOLE_h
#define __PRAGMA_CONSOLE_h

#define ANSI_ATTR_RESET			0
#define ANSI_BRIGHT			1
#define ANSI_DIM			2
#define ANSI_UNDERSCORE			4
#define ANSI_BLINK			5
#define ANSI_REVERSE			7
#define ANSI_HIDDEN			8
#define ANSI_NOUNDERSCORE		24
#define ANSI_NOREVERSE			27

#define ANSI_FOREGROUND_BLACK		30
#define ANSI_FOREGROUND_RED		31
#define ANSI_FOREGROUND_GREEN		32
#define ANSI_FOREGROUND_YELLOW		33
#define ANSI_FOREGROUND_BLUE		34
#define ANSI_FOREGROUND_MAGENTA		35
#define ANSI_FOREGROUND_CYAN		36
#define ANSI_FOREGROUND_WHITE		37
#define ANSI_DEFAULT_FOREGROUND		39
#define ANSI_BACKGROUND_BLACK		40
#define ANSI_BACKGROUND_RED		41
#define ANSI_BACKGROUND_GREEN		42
#define ANSI_BACKGROUND_YELLOW		43
#define ANSI_BACKGROUND_BLUE		44
#define ANSI_BACKGROUND_MAGENTA		45
#define ANSI_BACKGROUND_CYAN		46
#define ANSI_BACKGROUND_WHITE		47
#define ANSI_DEFAULT_BACKGROUND		49
#define ANSI_BACKGROUND_BRIGHT		128

#define TAB_LENGTH			4
#define TAB_CHAR			'\t'
#define TAB_SPACE			"    "

#define true TRUE
#define false FALSE
#define bool BOOL

#ifndef ENABLE_VIRTUAL_TERMINAL_PROCESSING
#define ENABLE_VIRTUAL_TERMINAL_PROCESSING  0x4
#endif

#ifndef ENABLE_VIRTUAL_TERMINAL_INPUT
#define ENABLE_VIRTUAL_TERMINAL_INPUT 0x0200
#endif

#ifndef DISABLE_NEWLINE_AUTO_RETURN
#define DISABLE_NEWLINE_AUTO_RETURN 0x8
#endif

//int ConWriteString(char* pszString, int cbString);

#endif
