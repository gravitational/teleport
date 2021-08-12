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
/* tncon.h
 * 
 * Contains terminal emulation console related key definition
 *
 */ 
#ifndef __TNCON_H
#define __TNCON_H

#define UP_ARROW                    "\x1b[A"
#define DOWN_ARROW                  "\x1b[B"
#define RIGHT_ARROW                 "\x1b[C"
#define LEFT_ARROW                  "\x1b[D"

#define APP_UP_ARROW                "\x1bOA"
#define APP_DOWN_ARROW              "\x1bOB"
#define APP_RIGHT_ARROW             "\x1bOC"
#define APP_LEFT_ARROW              "\x1bOD"

#define FIND_KEY                    "\x1b[1~"
#define INSERT_KEY                  "\x1b[2~"
#define REMOVE_KEY                  "\x1b[3~"
#define SELECT_KEY                  "\x1b[4~"
#define PREV_KEY                    "\x1b[5~"
#define NEXT_KEY                    "\x1b[6~"
#define SHIFT_TAB_KEY               "\x1b[~"
#define SHIFT_ALT_Q                 "\x1b?"
#define ESCAPE_KEY		    "\x1b"
#define BACKSPACE_KEY               "\x7f"

// VT100 Function Key's
#define VT100_PF1_KEY               "\x1bO2"
#define VT100_PF2_KEY               "\x1bO3"
#define VT100_PF3_KEY               "\x1bO4"
#define VT100_PF4_KEY               "\x1bO5"
#define VT100_PF5_KEY               "\x1bO6"
#define VT100_PF6_KEY               "\x1bO7"
#define VT100_PF7_KEY               "\x1bO8"
#define VT100_PF8_KEY               "\x1bO9"
#define VT100_PF9_KEY               "\x1bO:"
#define VT100_PF10_KEY              "\x1bO;"

// VT420 Key's
#define PF1_KEY                     "\x1b[11~"
#define PF2_KEY                     "\x1b[12~"
#define PF3_KEY                     "\x1b[13~"
#define PF4_KEY                     "\x1b[14~"
#define PF5_KEY                     "\x1b[15~"
#define PF6_KEY                     "\x1b[17~"
#define PF7_KEY                     "\x1b[18~"
#define PF8_KEY                     "\x1b[19~"
#define PF9_KEY                     "\x1b[20~"
#define PF10_KEY                    "\x1b[21~"
#define PF11_KEY                    "\x1b[23~"
#define PF12_KEY                    "\x1b[24~"

#define SHIFT_PF1_KEY               "\x1b[11;2~"
#define SHIFT_PF2_KEY               "\x1b[12;2~"
#define SHIFT_PF3_KEY               "\x1b[13;2~"
#define SHIFT_PF4_KEY               "\x1b[14;2~"
#define SHIFT_PF5_KEY               "\x1b[15;2~"
#define SHIFT_PF6_KEY               "\x1b[17;2~"
#define SHIFT_PF7_KEY               "\x1b[18;2~"
#define SHIFT_PF8_KEY               "\x1b[19;2~"
#define SHIFT_PF9_KEY               "\x1b[20;2~"
#define SHIFT_PF10_KEY              "\x1b[21;2~"
#define SHIFT_PF11_KEY              "\x1b[23;2~"
#define SHIFT_PF12_KEY              "\x1b[24;2~"

#define ALT_PF1_KEY                 "\x1b[11;3~"
#define ALT_PF2_KEY                 "\x1b[12;3~"
#define ALT_PF3_KEY                 "\x1b[13;3~"
#define ALT_PF4_KEY                 "\x1b[14;3~"
#define ALT_PF5_KEY                 "\x1b[15;3~"
#define ALT_PF6_KEY                 "\x1b[17;3~"
#define ALT_PF7_KEY                 "\x1b[18;3~"
#define ALT_PF8_KEY                 "\x1b[19;3~"
#define ALT_PF9_KEY                 "\x1b[20;3~"
#define ALT_PF10_KEY                "\x1b[21;3~"
#define ALT_PF11_KEY                "\x1b[23;3~"
#define ALT_PF12_KEY                "\x1b[24;3~"

#define CTRL_PF1_KEY                "\x1b[11;5~"
#define CTRL_PF2_KEY                "\x1b[12;5~"
#define CTRL_PF3_KEY                "\x1b[13;5~"
#define CTRL_PF4_KEY                "\x1b[14;5~"
#define CTRL_PF5_KEY                "\x1b[15;5~"
#define CTRL_PF6_KEY                "\x1b[17;5~"
#define CTRL_PF7_KEY                "\x1b[18;5~"
#define CTRL_PF8_KEY                "\x1b[19;5~"
#define CTRL_PF9_KEY                "\x1b[20;5~"
#define CTRL_PF10_KEY               "\x1b[21;5~"
#define CTRL_PF11_KEY               "\x1b[23;5~"
#define CTRL_PF12_KEY               "\x1b[24;5~"

#define SHIFT_CTRL_PF1_KEY          "\x1b[11;6~"
#define SHIFT_CTRL_PF2_KEY          "\x1b[12;6~"
#define SHIFT_CTRL_PF3_KEY          "\x1b[13;6~"
#define SHIFT_CTRL_PF4_KEY          "\x1b[14;6~"
#define SHIFT_CTRL_PF5_KEY          "\x1b[15;6~"
#define SHIFT_CTRL_PF6_KEY          "\x1b[17;6~"
#define SHIFT_CTRL_PF7_KEY          "\x1b[18;6~"
#define SHIFT_CTRL_PF8_KEY          "\x1b[19;6~"
#define SHIFT_CTRL_PF9_KEY          "\x1b[20;6~"
#define SHIFT_CTRL_PF10_KEY         "\x1b[21;6~"
#define SHIFT_CTRL_PF11_KEY         "\x1b[23;6~"
#define SHIFT_CTRL_PF12_KEY         "\x1b[24;6~"

#define SHIFT_ALT_PF1_KEY           "\x1b[11;4~"
#define SHIFT_ALT_PF2_KEY           "\x1b[12;4~"
#define SHIFT_ALT_PF3_KEY           "\x1b[13;4~"
#define SHIFT_ALT_PF4_KEY           "\x1b[14;4~"
#define SHIFT_ALT_PF5_KEY           "\x1b[15;4~"
#define SHIFT_ALT_PF6_KEY           "\x1b[17;4~"
#define SHIFT_ALT_PF7_KEY           "\x1b[18;4~"
#define SHIFT_ALT_PF8_KEY           "\x1b[19;4~"
#define SHIFT_ALT_PF9_KEY           "\x1b[20;4~"
#define SHIFT_ALT_PF10_KEY          "\x1b[21;4~"
#define SHIFT_ALT_PF11_KEY          "\x1b[23;4~"
#define SHIFT_ALT_PF12_KEY          "\x1b[24;4~"

#define ALT_CTRL_PF1_KEY            "\x1b[11;7~"
#define ALT_CTRL_PF2_KEY            "\x1b[12;7~"
#define ALT_CTRL_PF3_KEY            "\x1b[13;7~"
#define ALT_CTRL_PF4_KEY            "\x1b[14;7~"
#define ALT_CTRL_PF5_KEY            "\x1b[15;7~"
#define ALT_CTRL_PF6_KEY            "\x1b[17;7~"
#define ALT_CTRL_PF7_KEY            "\x1b[18;7~"
#define ALT_CTRL_PF8_KEY            "\x1b[19;7~"
#define ALT_CTRL_PF9_KEY            "\x1b[20;7~"
#define ALT_CTRL_PF10_KEY           "\x1b[21;7~"
#define ALT_CTRL_PF11_KEY           "\x1b[23;7~"
#define ALT_CTRL_PF12_KEY           "\x1b[24;7~"

#define SHIFT_ALT_CTRL_PF1_KEY      "\x1b[11;8~"
#define SHIFT_ALT_CTRL_PF2_KEY      "\x1b[12;8~"
#define SHIFT_ALT_CTRL_PF3_KEY      "\x1b[13;8~"
#define SHIFT_ALT_CTRL_PF4_KEY      "\x1b[14;8~"
#define SHIFT_ALT_CTRL_PF5_KEY      "\x1b[15;8~"
#define SHIFT_ALT_CTRL_PF6_KEY      "\x1b[17;8~"
#define SHIFT_ALT_CTRL_PF7_KEY      "\x1b[18;8~"
#define SHIFT_ALT_CTRL_PF8_KEY      "\x1b[19;8~"
#define SHIFT_ALT_CTRL_PF9_KEY      "\x1b[20;8~"
#define SHIFT_ALT_CTRL_PF10_KEY     "\x1b[21;8~"
#define SHIFT_ALT_CTRL_PF11_KEY     "\x1b[23;8~"
#define SHIFT_ALT_CTRL_PF12_KEY     "\x1b[24;8~"

/* XTERM (https://github.com/mintty/mintty/wiki/Keycodes#function-keys) */
#define XTERM_PF1_KEY               "\x1bOP"
#define XTERM_PF2_KEY               "\x1bOQ"
#define XTERM_PF3_KEY               "\x1bOR"
#define XTERM_PF4_KEY               "\x1bOS"

#define XTERM_SHIFT_PF1_KEY         "\x1b[1;2P"
#define XTERM_SHIFT_PF2_KEY         "\x1b[1;2Q"
#define XTERM_SHIFT_PF3_KEY         "\x1b[1;2R"
#define XTERM_SHIFT_PF4_KEY         "\x1b[1;2S"

#define XTERM_ALT_PF1_KEY           "\x1b[1;3P"
#define XTERM_ALT_PF2_KEY           "\x1b[1;3Q"
#define XTERM_ALT_PF3_KEY           "\x1b[1;3R"
#define XTERM_ALT_PF4_KEY           "\x1b[1;3S"

#define XTERM_CTRL_PF1_KEY          "\x1b[1;5P"
#define XTERM_CTRL_PF2_KEY          "\x1b[1;5Q"
#define XTERM_CTRL_PF3_KEY          "\x1b[1;5R"
#define XTERM_CTRL_PF4_KEY          "\x1b[1;5S"

#define XTERM_SHIFT_ALT_PF1_KEY     "\x1b[1;4P"
#define XTERM_SHIFT_ALT_PF2_KEY     "\x1b[1;4Q"
#define XTERM_SHIFT_ALT_PF3_KEY     "\x1b[1;4R"
#define XTERM_SHIFT_ALT_PF4_KEY     "\x1b[1;4S"

#define XTERM_SHIFT_CTRL_PF1_KEY    "\x1b[1;6P"
#define XTERM_SHIFT_CTRL_PF2_KEY    "\x1b[1;6Q"
#define XTERM_SHIFT_CTRL_PF3_KEY    "\x1b[1;6R"
#define XTERM_SHIFT_CTRL_PF4_KEY    "\x1b[1;6S"

#define XTERM_ALT_CTRL_PF1_KEY      "\x1b[1;7P"
#define XTERM_ALT_CTRL_PF2_KEY      "\x1b[1;7Q"
#define XTERM_ALT_CTRL_PF3_KEY      "\x1b[1;7R"
#define XTERM_ALT_CTRL_PF4_KEY      "\x1b[1;7S"

#define XTERM_SHIFT_ALT_CTRL_PF1_KEY      "\x1b[1;8P"
#define XTERM_SHIFT_ALT_CTRL_PF2_KEY      "\x1b[1;8Q"
#define XTERM_SHIFT_ALT_CTRL_PF3_KEY      "\x1b[1;8R"
#define XTERM_SHIFT_ALT_CTRL_PF4_KEY      "\x1b[1;8S"

#define TERMINAL_ID                 "\x1b[?1;2c"
#define STATUS_REPORT               "\x1b[2;5R"
#define CURSOR_REPORT_FORMAT_STRING "\x1b[%d;%dR"
#define VT52_TERMINAL_ID            "\x1b/Z"

// NOTE: Exported for use in tncon.go.
void ReadInputContinuous();

#endif
