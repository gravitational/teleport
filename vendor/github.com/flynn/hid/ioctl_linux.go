package hid

import "syscall"

// This file is https://github.com/wolfeidau/gioctl/blob/0a268ca608219d1d45cfcc50ca4dbfe232baaf0d/ioctl.go
//
// Copyright (c) 2014 Mark Wolfe and licenced under the MIT licence. All rights
// not explicitly granted in the MIT license are reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

const (
	typeBits      = 8
	numberBits    = 8
	sizeBits      = 14
	directionBits = 2

	typeMask      = (1 << typeBits) - 1
	numberMask    = (1 << numberBits) - 1
	sizeMask      = (1 << sizeBits) - 1
	directionMask = (1 << directionBits) - 1

	directionNone  = 0
	directionWrite = 1
	directionRead  = 2

	numberShift    = 0
	typeShift      = numberShift + numberBits
	sizeShift      = typeShift + typeBits
	directionShift = sizeShift + sizeBits
)

func ioc(dir, t, nr, size uintptr) uintptr {
	return (dir << directionShift) | (t << typeShift) | (nr << numberShift) | (size << sizeShift)
}

// io used for a simple ioctl that sends nothing but the type and number, and receives back nothing but an (integer) retval.
func io(t, nr uintptr) uintptr {
	return ioc(directionNone, t, nr, 0)
}

// ioR used for an ioctl that reads data from the device driver. The driver will be allowed to return sizeof(data_type) bytes to the user.
func ioR(t, nr, size uintptr) uintptr {
	return ioc(directionRead, t, nr, size)
}

// ioW used for an ioctl that writes data to the device driver.
func ioW(t, nr, size uintptr) uintptr {
	return ioc(directionWrite, t, nr, size)
}

// ioRW  a combination of IoR and IoW. That is, data is both written to the driver and then read back from the driver by the client.
func ioRW(t, nr, size uintptr) uintptr {
	return ioc(directionRead|directionWrite, t, nr, size)
}

// ioctl simplified ioct call
func ioctl(fd, op, arg uintptr) error {
	_, _, ep := syscall.Syscall(syscall.SYS_IOCTL, fd, op, arg)
	if ep != 0 {
		return syscall.Errno(ep)
	}
	return nil
}
