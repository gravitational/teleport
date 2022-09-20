// +build go1.9,!go1.10,darwin

package hid

/*
#include <IOKit/hid/IOHIDManager.h>
#include <CoreFoundation/CoreFoundation.h>
 */
import "C"

var nilCfStringRef C.CFStringRef= nil
var nilCfTypeRef C.CFTypeRef = nil
var nilCfSetRef C.CFSetRef = nil
var nilIOHIDDeviceRef C.IOHIDDeviceRef = nil
var nilCfDictionaryRef C.CFDictionaryRef = nil
