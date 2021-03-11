// +build go1.10,darwin

package hid

/*
#include <IOKit/hid/IOHIDManager.h>
#include <CoreFoundation/CoreFoundation.h>
*/
import "C"

var nilCfStringRef C.CFStringRef= 0
var nilCfTypeRef C.CFTypeRef = 0
var nilCfSetRef C.CFSetRef = 0
var nilIOHIDDeviceRef C.IOHIDDeviceRef = 0
var nilCfDictionaryRef C.CFDictionaryRef = 0
