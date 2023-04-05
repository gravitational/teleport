package main

/*
#include <stdlib.h>

*/
import "C"

import (
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"unsafe"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
	"gopkg.in/natefinch/lumberjack.v2"
)

func init() {
	logsDir := "C:\\Windows\\Logs"
	file := "teleport.cp.txt"
	exe, err := os.Executable()
	if err == nil && strings.Contains(exe, "lsass") {
		file = "teleport.ap.txt"
	}
	os.MkdirAll(logsDir, 0600)
	writer := &lumberjack.Logger{
		Filename:   "C:\\Windows\\Logs\\" + file,
		MaxSize:    500, // megabytes
		MaxBackups: 3,
		MaxAge:     28, // days
	}
	if strings.Contains(file, "ap") {
		writer.Rotate()
	}
	log.SetOutput(writer)
	log.SetReportCaller(true)
	log.SetFormatter(&log.JSONFormatter{
		CallerPrettyfier: func(frame *runtime.Frame) (function string, file string) {
			fileName := path.Base(frame.File) + ":" + strconv.Itoa(frame.Line)
			//return frame.Function, fileName
			return "", fileName
		},
	})
}

var refCount atomic.Int32

//export DllAddRef
func DllAddRef() {
	refCount.Add(1)
}

//export DllRelease
func DllRelease() {
	refCount.Add(-1)
}

//export DllCanUnloadNow
func DllCanUnloadNow() uint32 {
	if refCount.Load() > 0 {
		return S_OK
	}
	return E_FAIL
}

//export DllGetClassObject
func DllGetClassObject(rclsid, riid unsafe.Pointer, ppv *unsafe.Pointer) uint32 {
	wrclsid := (*windows.GUID)(rclsid)
	wriid := (*windows.GUID)(riid)

	if *wrclsid != CredentialProviderGUID || *wriid != IClassFactoryGUID {
		*ppv = nil
		return uint32(windows.CLASS_E_CLASSNOTAVAILABLE)
	}

	*ppv = C.calloc(1, C.ulonglong(unsafe.Sizeof(Com{})))
	com := (*Com)(*ppv)
	com.count.Add(1)
	com.vTable = &classFactoryVTable[0]
	com.guid = IClassFactoryGUID
	return S_OK
}

func main() {
}
