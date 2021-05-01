package hid

/*
#cgo LDFLAGS: -L . -L/usr/local/lib -framework CoreFoundation -framework IOKit

#include <IOKit/hid/IOHIDManager.h>
#include <CoreFoundation/CoreFoundation.h>


static inline CFIndex cfstring_utf8_length(CFStringRef str, CFIndex *need) {
  CFIndex n, usedBufLen;
  CFRange rng = CFRangeMake(0, CFStringGetLength(str));

  return CFStringGetBytes(str, rng, kCFStringEncodingUTF8, 0, 0, NULL, 0, need);
}

void deviceUnplugged(IOHIDDeviceRef osd, IOReturn ret, void *dev);

void reportCallback(void *context, IOReturn result, void *sender, IOHIDReportType report_type, uint32_t report_id, uint8_t *report, CFIndex report_length);

*/
import "C"

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"unsafe"
)

func ioReturnToErr(ret C.IOReturn) error {
	switch ret {
	case C.kIOReturnSuccess:
		return nil
	case C.kIOReturnError:
		return errors.New("hid: general error")
	case C.kIOReturnNoMemory:
		return errors.New("hid: can't allocate memory")
	case C.kIOReturnNoResources:
		return errors.New("hid: resource shortage")
	case C.kIOReturnIPCError:
		return errors.New("hid: error during IPC")
	case C.kIOReturnNoDevice:
		return errors.New("hid: no such device")
	case C.kIOReturnNotPrivileged:
		return errors.New("hid: privilege violation")
	case C.kIOReturnBadArgument:
		return errors.New("hid: invalid argument")
	case C.kIOReturnLockedRead:
		return errors.New("hid: device read locked")
	case C.kIOReturnLockedWrite:
		return errors.New("hid: device write locked")
	case C.kIOReturnExclusiveAccess:
		return errors.New("hid: exclusive access and device already open")
	case C.kIOReturnBadMessageID:
		return errors.New("hid: sent/received messages had different msg_id")
	case C.kIOReturnUnsupported:
		return errors.New("hid: unsupported function")
	case C.kIOReturnVMError:
		return errors.New("hid: misc. VM failure")
	case C.kIOReturnInternalError:
		return errors.New("hid: internal error")
	case C.kIOReturnIOError:
		return errors.New("hid: general I/O error")
	case C.kIOReturnCannotLock:
		return errors.New("hid: can't acquire lock")
	case C.kIOReturnNotOpen:
		return errors.New("hid: device not open")
	case C.kIOReturnNotReadable:
		return errors.New("hid: read not supported")
	case C.kIOReturnNotWritable:
		return errors.New("hid: write not supported")
	case C.kIOReturnNotAligned:
		return errors.New("hid: alignment error")
	case C.kIOReturnBadMedia:
		return errors.New("hid: media Error")
	case C.kIOReturnStillOpen:
		return errors.New("hid: device(s) still open")
	case C.kIOReturnRLDError:
		return errors.New("hid: rld failure")
	case C.kIOReturnDMAError:
		return errors.New("hid: DMA failure")
	case C.kIOReturnBusy:
		return errors.New("hid: device Busy")
	case C.kIOReturnTimeout:
		return errors.New("hid: i/o timeout")
	case C.kIOReturnOffline:
		return errors.New("hid: device offline")
	case C.kIOReturnNotReady:
		return errors.New("hid: not ready")
	case C.kIOReturnNotAttached:
		return errors.New("hid: device not attached")
	case C.kIOReturnNoChannels:
		return errors.New("hid: no DMA channels left")
	case C.kIOReturnNoSpace:
		return errors.New("hid: no space for data")
	case C.kIOReturnPortExists:
		return errors.New("hid: port already exists")
	case C.kIOReturnCannotWire:
		return errors.New("hid: can't wire down physical memory")
	case C.kIOReturnNoInterrupt:
		return errors.New("hid: no interrupt attached")
	case C.kIOReturnNoFrames:
		return errors.New("hid: no DMA frames enqueued")
	case C.kIOReturnMessageTooLarge:
		return errors.New("hid: oversized msg received on interrupt port")
	case C.kIOReturnNotPermitted:
		return errors.New("hid: not permitted")
	case C.kIOReturnNoPower:
		return errors.New("hid: no power to device")
	case C.kIOReturnNoMedia:
		return errors.New("hid: media not present")
	case C.kIOReturnUnformattedMedia:
		return errors.New("hid: media not formatted")
	case C.kIOReturnUnsupportedMode:
		return errors.New("hid: no such mode")
	case C.kIOReturnUnderrun:
		return errors.New("hid: data underrun")
	case C.kIOReturnOverrun:
		return errors.New("hid: data overrun")
	case C.kIOReturnDeviceError:
		return errors.New("hid: the device is not working properly!")
	case C.kIOReturnNoCompletion:
		return errors.New("hid: a completion routine is required")
	case C.kIOReturnAborted:
		return errors.New("hid: operation aborted")
	case C.kIOReturnNoBandwidth:
		return errors.New("hid: bus bandwidth would be exceeded")
	case C.kIOReturnNotResponding:
		return errors.New("hid: device not responding")
	case C.kIOReturnIsoTooOld:
		return errors.New("hid: isochronous I/O request for distant past!")
	case C.kIOReturnIsoTooNew:
		return errors.New("hid: isochronous I/O request for distant future")
	case C.kIOReturnNotFound:
		return errors.New("hid: data was not found")
	default:
		return errors.New("hid: unknown error")
	}
}

var deviceCtxMtx sync.Mutex
var deviceCtx = make(map[C.IOHIDDeviceRef]*osxDevice)

type cleanupDeviceManagerFn func()
type osxDevice struct {
	mtx          sync.Mutex
	osDevice     C.IOHIDDeviceRef
	disconnected bool
	closeDM      cleanupDeviceManagerFn

	readSetup sync.Once
	readCh    chan []byte
	readErr   error
	readBuf   []byte
	runLoop   C.CFRunLoopRef
}

func cfstring(s string) C.CFStringRef {
	n := C.CFIndex(len(s))
	return C.CFStringCreateWithBytes(C.kCFAllocatorDefault, *(**C.UInt8)(unsafe.Pointer(&s)), n, C.kCFStringEncodingUTF8, 0)
}

func gostring(cfs C.CFStringRef) string {
	if cfs == nilCfStringRef {
		return ""
	}

	var usedBufLen C.CFIndex
	n := C.cfstring_utf8_length(cfs, &usedBufLen)
	if n <= 0 {
		return ""
	}
	rng := C.CFRange{location: C.CFIndex(0), length: n}
	buf := make([]byte, int(usedBufLen))

	bufp := unsafe.Pointer(&buf[0])
	C.CFStringGetBytes(cfs, rng, C.kCFStringEncodingUTF8, 0, 0, (*C.UInt8)(bufp), C.CFIndex(len(buf)), &usedBufLen)

	sh := &reflect.StringHeader{
		Data: uintptr(bufp),
		Len:  int(usedBufLen),
	}
	return *(*string)(unsafe.Pointer(sh))
}

func getIntProp(device C.IOHIDDeviceRef, key C.CFStringRef) int32 {
	var value int32
	ref := C.IOHIDDeviceGetProperty(device, key)
	if ref == nilCfTypeRef {
		return 0
	}
	if C.CFGetTypeID(ref) != C.CFNumberGetTypeID() {
		return 0
	}
	C.CFNumberGetValue(C.CFNumberRef(ref), C.kCFNumberSInt32Type, unsafe.Pointer(&value))
	return value
}

func getStringProp(device C.IOHIDDeviceRef, key C.CFStringRef) string {
	s := C.IOHIDDeviceGetProperty(device, key)
	return gostring(C.CFStringRef(s))
}

func getPath(osDev C.IOHIDDeviceRef) string {
	return fmt.Sprintf("%s_%04x_%04x_%08x",
		getStringProp(osDev, cfstring(C.kIOHIDTransportKey)),
		uint16(getIntProp(osDev, cfstring(C.kIOHIDVendorIDKey))),
		uint16(getIntProp(osDev, cfstring(C.kIOHIDProductIDKey))),
		uint32(getIntProp(osDev, cfstring(C.kIOHIDLocationIDKey))))
}

func iterateDevices(action func(device C.IOHIDDeviceRef) bool) cleanupDeviceManagerFn {
	var mgr C.IOHIDManagerRef
	mgr = C.IOHIDManagerCreate(C.kCFAllocatorDefault, C.kIOHIDOptionsTypeNone)
	C.IOHIDManagerSetDeviceMatching(mgr, nilCfDictionaryRef)
	C.IOHIDManagerOpen(mgr, C.kIOHIDOptionsTypeNone)

	var allDevicesSet C.CFSetRef
	allDevicesSet = C.IOHIDManagerCopyDevices(mgr)
	if allDevicesSet == nilCfSetRef {
		return func() {}
	}
	defer C.CFRelease((C.CFTypeRef)(allDevicesSet))
	devCnt := C.CFSetGetCount(allDevicesSet)
	allDevices := make([]unsafe.Pointer, uint64(devCnt))
	C.CFSetGetValues(allDevicesSet, &allDevices[0])

	for _, pDev := range allDevices {
		if !action(C.IOHIDDeviceRef(pDev)) {
			break
		}
	}
	return func() {
		C.IOHIDManagerClose(mgr, C.kIOHIDOptionsTypeNone)
		C.CFRelease(C.CFTypeRef(mgr))
	}
}

func Devices() ([]*DeviceInfo, error) {
	var result []*DeviceInfo
	iterateDevices(func(device C.IOHIDDeviceRef) bool {
		result = append(result, &DeviceInfo{
			VendorID:           uint16(getIntProp(device, cfstring(C.kIOHIDVendorIDKey))),
			ProductID:          uint16(getIntProp(device, cfstring(C.kIOHIDProductIDKey))),
			VersionNumber:      uint16(getIntProp(device, cfstring(C.kIOHIDVersionNumberKey))),
			Manufacturer:       getStringProp(device, cfstring(C.kIOHIDManufacturerKey)),
			Product:            getStringProp(device, cfstring(C.kIOHIDProductKey)),
			UsagePage:          uint16(getIntProp(device, cfstring(C.kIOHIDPrimaryUsagePageKey))),
			Usage:              uint16(getIntProp(device, cfstring(C.kIOHIDPrimaryUsageKey))),
			InputReportLength:  uint16(getIntProp(device, cfstring(C.kIOHIDMaxInputReportSizeKey))),
			OutputReportLength: uint16(getIntProp(device, cfstring(C.kIOHIDMaxOutputReportSizeKey))),
			Path:               getPath(device),
		})
		return true
	})()
	return result, nil
}

func ByPath(path string) (*DeviceInfo, error) {
	devices, err := Devices()
	if err != nil {
		return nil, err
	}
	for _, d := range devices {
		if d.Path == path {
			return d, nil
		}
	}
	return nil, errors.New("hid: device not found")
}

func (di *DeviceInfo) Open() (Device, error) {
	err := errors.New("hid: device not found")
	var dev *osxDevice
	closeDM := iterateDevices(func(device C.IOHIDDeviceRef) bool {
		if getPath(device) == di.Path {
			res := C.IOHIDDeviceOpen(device, C.kIOHIDOptionsTypeSeizeDevice)
			if res == C.kIOReturnSuccess {
				C.CFRetain(C.CFTypeRef(device))
				dev = &osxDevice{osDevice: device}
				err = nil
				deviceCtxMtx.Lock()
				deviceCtx[device] = dev
				deviceCtxMtx.Unlock()
				C.IOHIDDeviceRegisterRemovalCallback(device, (C.IOHIDCallback)(unsafe.Pointer(C.deviceUnplugged)), unsafe.Pointer(device))
			} else {
				err = ioReturnToErr(res)
			}
			return false
		}
		return true
	})
	if dev != nil {
		dev.closeDM = closeDM
		dev.readBuf = make([]byte, int(di.InputReportLength))
	}

	return dev, err
}

//export deviceUnplugged
func deviceUnplugged(osdev C.IOHIDDeviceRef, result C.IOReturn, dev unsafe.Pointer) {
	deviceCtxMtx.Lock()
	od := deviceCtx[C.IOHIDDeviceRef(dev)]
	deviceCtxMtx.Unlock()
	od.readErr = errors.New("hid: device unplugged")
	od.close(true)
}

func (dev *osxDevice) Close() {
	dev.readErr = errors.New("hid: device closed")
	dev.close(false)
}

func (dev *osxDevice) close(disconnected bool) {
	dev.mtx.Lock()
	defer dev.mtx.Unlock()

	if dev.disconnected {
		return
	}

	if dev.readCh != nil {
		if !disconnected {
			C.IOHIDDeviceRegisterInputReportCallback(dev.osDevice, (*C.uint8_t)(&dev.readBuf[0]), C.CFIndex(len(dev.readBuf)), nil, unsafe.Pointer(dev.osDevice))
			C.IOHIDDeviceUnscheduleFromRunLoop(dev.osDevice, dev.runLoop, C.kCFRunLoopDefaultMode)
		}
		C.CFRunLoopStop(dev.runLoop)
	}
	if !disconnected {
		C.IOHIDDeviceRegisterRemovalCallback(dev.osDevice, nil, nil)
		C.IOHIDDeviceClose(dev.osDevice, C.kIOHIDOptionsTypeSeizeDevice)
	}

	deviceCtxMtx.Lock()
	delete(deviceCtx, dev.osDevice)
	deviceCtxMtx.Unlock()
	C.CFRelease(C.CFTypeRef(dev.osDevice))
	dev.osDevice = nilIOHIDDeviceRef
	dev.closeDM()
	dev.disconnected = true
}

func (dev *osxDevice) setReport(typ C.IOHIDReportType, data []byte) error {
	dev.mtx.Lock()
	defer dev.mtx.Unlock()

	if dev.disconnected {
		return errors.New("hid: device disconnected")
	}

	reportNo := int32(data[0])
	if reportNo == 0 {
		data = data[1:]
	}

	res := C.IOHIDDeviceSetReport(dev.osDevice, typ, C.CFIndex(reportNo), (*C.uint8_t)(&data[0]), C.CFIndex(len(data)))
	if res != C.kIOReturnSuccess {
		return ioReturnToErr(res)
	}
	return nil
}

func (dev *osxDevice) Write(data []byte) error {
	return dev.setReport(C.kIOHIDReportTypeOutput, data)
}

func (dev *osxDevice) ReadCh() <-chan []byte {
	dev.readSetup.Do(dev.startReadThread)
	return dev.readCh
}

func (dev *osxDevice) startReadThread() {
	dev.mtx.Lock()
	dev.readCh = make(chan []byte, 30)
	dev.mtx.Unlock()

	go func() {
		runtime.LockOSThread()
		dev.mtx.Lock()
		dev.runLoop = C.CFRunLoopGetCurrent()
		C.IOHIDDeviceScheduleWithRunLoop(dev.osDevice, dev.runLoop, C.kCFRunLoopDefaultMode)
		C.IOHIDDeviceRegisterInputReportCallback(dev.osDevice, (*C.uint8_t)(&dev.readBuf[0]), C.CFIndex(len(dev.readBuf)), (C.IOHIDReportCallback)(unsafe.Pointer(C.reportCallback)), unsafe.Pointer(dev.osDevice))
		dev.mtx.Unlock()
		C.CFRunLoopRun()
		close(dev.readCh)
	}()
}

func (dev *osxDevice) ReadError() error {
	return dev.readErr
}

//export reportCallback
func reportCallback(context unsafe.Pointer, result C.IOReturn, sender unsafe.Pointer, reportType C.IOHIDReportType, reportID uint32, report *C.uint8_t, reportLength C.CFIndex) {
	deviceCtxMtx.Lock()
	dev, ok := deviceCtx[(C.IOHIDDeviceRef)(context)]
	deviceCtxMtx.Unlock()
	if !ok {
		return
	}
	data := C.GoBytes(unsafe.Pointer(report), C.int(reportLength))

	// readCh is buffered, drop the data if we can't send to avoid blocking the
	// run loop
	select {
	case dev.readCh <- data:
	default:
	}
}
