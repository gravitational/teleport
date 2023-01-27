// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package protocol

import "bytes"

// These internal MongoDB wire message parsing methods were adopted from:
//
// https://github.com/mongodb/mongo-go-driver/blob/v1.7.2/x/mongo/driver/wiremessage/wiremessage.go

func readInt32(src []byte) (int32, []byte, bool) {
	if len(src) < 4 {
		return 0, src, false
	}
	return (int32(src[0]) | int32(src[1])<<8 | int32(src[2])<<16 | int32(src[3])<<24), src[4:], true
}

func appendInt32(dst []byte, i32 int32) []byte {
	return append(dst, byte(i32), byte(i32>>8), byte(i32>>16), byte(i32>>24))
}

func readInt64(src []byte) (int64, []byte, bool) {
	if len(src) < 8 {
		return 0, src, false
	}
	i64 := (int64(src[0]) | int64(src[1])<<8 | int64(src[2])<<16 | int64(src[3])<<24 |
		int64(src[4])<<32 | int64(src[5])<<40 | int64(src[6])<<48 | int64(src[7])<<56)
	return i64, src[8:], true
}

func appendInt64(b []byte, i int64) []byte {
	return append(b, byte(i), byte(i>>8), byte(i>>16), byte(i>>24), byte(i>>32), byte(i>>40), byte(i>>48), byte(i>>56))
}

func readCString(src []byte) (string, []byte, bool) {
	idx := bytes.IndexByte(src, 0x00)
	if idx < 0 {
		return "", src, false
	}
	return string(src[:idx]), src[idx+1:], true
}

func appendCString(b []byte, str string) []byte {
	b = append(b, str...)
	return append(b, 0x00)
}
