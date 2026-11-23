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

/*
 * The key event translation below is ported from ReadConsoleForTermEmul in
 * https://github.com/PowerShell/openssh-portable/blob/0b73c4636d38f0c69424721d52d0e7752db99c81/contrib/win32/win32compat/tncon.c
 * which carries the following notice:
 *
 * Author: Ray Hayes <ray.hayes@microsoft.com>
 * ANSI TTY Reader - Maps Windows console input events to ANSI stream
 *
 * Author: Balu <bagajjal@microsoft.com>
 * Misc fixes and code cleanup
 *
 * Copyright (c) 2017 Microsoft Corp.
 * All rights reserved
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

package tncon

import (
	"unicode/utf16"
	"unicode/utf8"
)

// vkMenu is the virtual key code of the Alt key (VK_MENU).
const vkMenu = 0x12

// keyDecoder converts console key events to UTF-8 input. It is kept free of
// Windows APIs so that it can be tested on every platform.
type keyDecoder struct {
	// surrogates holds a UTF-16 surrogate pair that arrives in two key events.
	surrogates [2]uint16
}

// decode returns the UTF-8 bytes produced by a key event, or nil if the
// event produces no input.
//
// Key-up events are ignored, except for Alt. Windows reports characters
// entered with Alt+numpad codes on the Alt key-up event.
func (d *keyDecoder) decode(keyDown bool, virtualKeyCode, unicodeChar uint16) []byte {
	if !keyDown && virtualKeyCode != vkMenu {
		return nil
	}

	switch {
	case utf16.IsSurrogate(rune(unicodeChar)) && unicodeChar < 0xdc00:
		// High surrogate: wait for the low surrogate.
		d.surrogates[0] = unicodeChar
		return nil
	case utf16.IsSurrogate(rune(unicodeChar)):
		d.surrogates[1] = unicodeChar
	}

	if d.surrogates[0] != 0 && d.surrogates[1] != 0 {
		r := utf16.DecodeRune(rune(d.surrogates[0]), rune(d.surrogates[1]))
		d.surrogates = [2]uint16{}
		return utf8.AppendRune(nil, r)
	}

	if unicodeChar == 0 {
		return nil
	}

	// A lone surrogate is invalid, and is encoded as U+FFFD just like
	// WideCharToMultiByte does.
	return utf8.AppendRune(nil, utf16.Decode([]uint16{unicodeChar})[0])
}
