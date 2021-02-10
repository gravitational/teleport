package hid

/*
#cgo LDFLAGS: -lsetupapi -lhid

#ifdef __MINGW32__
#include <ntdef.h>
#endif

#include <windows.h>
#include <setupapi.h>
#include <hidsdi.h>
*/
import "C"

import (
	"errors"
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

type winDevice struct {
	handle syscall.Handle
	info   *DeviceInfo

	readSetup sync.Once
	readCh    chan []byte
	readErr   error
	readOl    *syscall.Overlapped
}

// returns the casted handle of the device
func (d *winDevice) h() C.HANDLE {
	return (C.HANDLE)((unsafe.Pointer)(d.handle))
}

// checks if the handle of the device is valid
func (d *winDevice) isValid() bool {
	return d.handle != syscall.InvalidHandle
}

func (d *winDevice) Close() {
	// cancel any pending reads and unblock read loop
	d.readErr = errors.New("hid: device closed")
	C.CancelIo(d.h())
	C.SetEvent(C.HANDLE(unsafe.Pointer(d.readOl.HEvent)))
	syscall.CloseHandle(d.readOl.HEvent)

	syscall.CloseHandle(d.handle)
	d.handle = syscall.InvalidHandle
}

func (d *winDevice) Write(data []byte) error {
	// first make sure we send the correct amount of data to the device
	outSize := int(d.info.OutputReportLength + 1)
	if len(data) != outSize {
		buf := make([]byte, outSize)
		copy(buf, data)
		data = buf
	}

	ol := new(syscall.Overlapped)
	if err := syscall.WriteFile(d.handle, data, nil, ol); err != nil {
		// IO Pending is ok we simply wait for it to finish a few lines below
		// all other errors should be reported.
		if err != syscall.ERROR_IO_PENDING {
			return err
		}
	}

	// now wait for the overlapped device access to finish.
	var written C.DWORD
	if C.GetOverlappedResult(d.h(), (*C.OVERLAPPED)((unsafe.Pointer)(ol)), &written, C.TRUE) == 0 {
		return syscall.GetLastError()
	}

	if int(written) != outSize {
		return errors.New("written bytes missmatch!")
	}
	return nil
}

type callCFn func(buf unsafe.Pointer, bufSize *C.DWORD) unsafe.Pointer

// simple helper function for this windows
// "call a function twice to get the amount of space that needs to be allocated" stuff
func getCString(fnCall callCFn) string {
	var requiredSize C.DWORD
	fnCall(nil, &requiredSize)
	if requiredSize <= 0 {
		return ""
	}

	buffer := C.malloc((C.size_t)(requiredSize))
	defer C.free(buffer)

	strPt := fnCall(buffer, &requiredSize)

	return C.GoString((*C.char)(strPt))
}

func openDevice(info *DeviceInfo, enumerate bool) (*winDevice, error) {
	access := uint32(syscall.GENERIC_WRITE | syscall.GENERIC_READ)
	shareMode := uint32(syscall.FILE_SHARE_READ)
	if enumerate {
		// if we just need a handle to get the device properties
		// we should not claim exclusive access on the device
		access = 0
		shareMode = uint32(syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE)
	}
	pPtr, err := syscall.UTF16PtrFromString(info.Path)
	if err != nil {
		return nil, err
	}

	hFile, err := syscall.CreateFile(pPtr, access, shareMode, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return nil, err
	} else {
		return &winDevice{
			handle: hFile,
			info:   info,
			readOl: &syscall.Overlapped{
				HEvent: syscall.Handle(C.CreateEvent(nil, C.FALSE, C.FALSE, nil)),
			},
		}, nil
	}
}

func getDeviceDetails(deviceInfoSet C.HDEVINFO, deviceInterfaceData *C.SP_DEVICE_INTERFACE_DATA) *DeviceInfo {
	devicePath := getCString(func(buffer unsafe.Pointer, size *C.DWORD) unsafe.Pointer {
		interfaceDetailData := (*C.SP_DEVICE_INTERFACE_DETAIL_DATA_A)(buffer)
		if interfaceDetailData != nil {
			interfaceDetailData.cbSize = C.DWORD(unsafe.Sizeof(interfaceDetailData))
		}
		C.SetupDiGetDeviceInterfaceDetailA(deviceInfoSet, deviceInterfaceData, interfaceDetailData, *size, size, nil)
		if interfaceDetailData != nil {
			return (unsafe.Pointer)(&interfaceDetailData.DevicePath[0])
		} else {
			return nil
		}
	})
	if devicePath == "" {
		return nil
	}

	// Make sure this device is of Setup Class "HIDClass" and has a driver bound to it.
	var i C.DWORD
	var devinfoData C.SP_DEVINFO_DATA
	devinfoData.cbSize = C.DWORD(unsafe.Sizeof(devinfoData))
	isHID := false
	for i = 0; ; i++ {
		if res := C.SetupDiEnumDeviceInfo(deviceInfoSet, i, &devinfoData); res == 0 {
			break
		}

		classStr := getCString(func(buffer unsafe.Pointer, size *C.DWORD) unsafe.Pointer {
			C.SetupDiGetDeviceRegistryPropertyA(deviceInfoSet, &devinfoData, C.SPDRP_CLASS, nil, (*C.BYTE)(buffer), *size, size)
			return buffer
		})

		if classStr == "HIDClass" {
			driverName := getCString(func(buffer unsafe.Pointer, size *C.DWORD) unsafe.Pointer {
				C.SetupDiGetDeviceRegistryPropertyA(deviceInfoSet, &devinfoData, C.SPDRP_DRIVER, nil, (*C.BYTE)(buffer), *size, size)
				return buffer
			})
			isHID = driverName != ""
			break
		}
	}

	if !isHID {
		return nil
	}
	d, _ := ByPath(devicePath)
	return d
}

// ByPath gets the device which is bound to the given path.
func ByPath(devicePath string) (*DeviceInfo, error) {
	devInfo := &DeviceInfo{Path: devicePath}
	dev, err := openDevice(devInfo, true)
	if err != nil {
		return nil, err
	}
	defer dev.Close()
	if !dev.isValid() {
		return nil, errors.New("Failed to open device")
	}

	var attrs C.HIDD_ATTRIBUTES
	attrs.Size = C.DWORD(unsafe.Sizeof(attrs))
	C.HidD_GetAttributes(dev.h(), &attrs)

	devInfo.VendorID = uint16(attrs.VendorID)
	devInfo.ProductID = uint16(attrs.ProductID)
	devInfo.VersionNumber = uint16(attrs.VersionNumber)

	const bufLen = 256
	buff := make([]uint16, bufLen)

	C.HidD_GetManufacturerString(dev.h(), (C.PVOID)(&buff[0]), bufLen)
	devInfo.Manufacturer = syscall.UTF16ToString(buff)

	C.HidD_GetProductString(dev.h(), (C.PVOID)(&buff[0]), bufLen)
	devInfo.Product = syscall.UTF16ToString(buff)

	var preparsedData C.PHIDP_PREPARSED_DATA
	if C.HidD_GetPreparsedData(dev.h(), &preparsedData) != 0 {
		var caps C.HIDP_CAPS

		if C.HidP_GetCaps(preparsedData, &caps) == C.HIDP_STATUS_SUCCESS {
			devInfo.UsagePage = uint16(caps.UsagePage)
			devInfo.Usage = uint16(caps.Usage)
			devInfo.InputReportLength = uint16(caps.InputReportByteLength - 1)
			devInfo.OutputReportLength = uint16(caps.OutputReportByteLength - 1)
		}

		C.HidD_FreePreparsedData(preparsedData)
	}

	return devInfo, nil
}

// Devices returns all HID devices which are connected to the system.
func Devices() ([]*DeviceInfo, error) {
	var result []*DeviceInfo
	var InterfaceClassGuid C.GUID
	C.HidD_GetHidGuid(&InterfaceClassGuid)
	deviceInfoSet := C.SetupDiGetClassDevsA(&InterfaceClassGuid, nil, nil, C.DIGCF_PRESENT|C.DIGCF_DEVICEINTERFACE)
	defer C.SetupDiDestroyDeviceInfoList(deviceInfoSet)

	var deviceIdx C.DWORD = 0
	var deviceInterfaceData C.SP_DEVICE_INTERFACE_DATA
	deviceInterfaceData.cbSize = C.DWORD(unsafe.Sizeof(deviceInterfaceData))

	for ; ; deviceIdx++ {
		res := C.SetupDiEnumDeviceInterfaces(deviceInfoSet, nil, &InterfaceClassGuid, deviceIdx, &deviceInterfaceData)
		if res == 0 {
			break
		}
		di := getDeviceDetails(deviceInfoSet, &deviceInterfaceData)
		if di != nil {
			result = append(result, di)
		}
	}
	return result, nil
}

// Open openes the device for read / write access.
func (di *DeviceInfo) Open() (Device, error) {
	d, err := openDevice(di, false)
	if err != nil {
		return nil, err
	}
	if d.isValid() {
		return d, nil
	} else {
		d.Close()
		err := syscall.GetLastError()
		if err == nil {
			err = errors.New("Unable to open device!")
		}
		return nil, err
	}
}

func (d *winDevice) ReadCh() <-chan []byte {
	d.readSetup.Do(func() {
		d.readCh = make(chan []byte, 30)
		go d.readThread()
	})
	return d.readCh
}

func (d *winDevice) ReadError() error {
	return d.readErr
}

func (d *winDevice) readThread() {
	defer close(d.readCh)

	for {
		buf := make([]byte, d.info.InputReportLength+1)
		C.ResetEvent(C.HANDLE(unsafe.Pointer(d.readOl.HEvent)))

		if err := syscall.ReadFile(d.handle, buf, nil, d.readOl); err != nil {
			if err != syscall.ERROR_IO_PENDING {
				if d.readErr == nil {
					d.readErr = err
				}
				return
			}
		}

		// Wait for the read to finish
		res := C.WaitForSingleObject(C.HANDLE(unsafe.Pointer(d.readOl.HEvent)), C.INFINITE)
		if res != C.WAIT_OBJECT_0 {
			if d.readErr == nil {
				d.readErr = fmt.Errorf("hid: unexpected read wait state %d", res)
			}
			return
		}

		var n C.DWORD
		if r := C.GetOverlappedResult(d.h(), (*C.OVERLAPPED)((unsafe.Pointer)(d.readOl)), &n, C.TRUE); r == 0 {
			if d.readErr == nil {
				d.readErr = fmt.Errorf("hid: unexpected read result state %d", r)
			}
			return
		}
		if n == 0 {
			if d.readErr == nil {
				d.readErr = errors.New("hid: zero byte read")
			}
			return
		}

		if buf[0] == 0 {
			// Report numbers are not being used, so remove zero to match other platforms
			buf = buf[1:]
		}

		select {
		case d.readCh <- buf[:int(n-1)]:
		default:
		}
	}

}
