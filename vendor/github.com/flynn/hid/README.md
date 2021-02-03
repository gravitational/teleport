# HID

A Go package to access Human Interface Devices. The platform specific parts of
this package are heavily based on [Signal 11's HIDAPI](https://github.com/signal11/hidapi).

## Supported operating systems

The following operating systems are supported targets 
(as used by [*$GOOS* environment variable](https://golang.org/doc/install/source#environment))

* darwin (uses native IOKit framework)
* linux (uses hidraw)
* windows (uses native Windows HID library)

## Known quirks for building on Windows 64-bit

For building this HID package, you need to have a gcc.exe in your *%PATH%* environment variable.
There are two tested GCC toolchains: [tdm-gcc](http://tdm-gcc.tdragon.net/)
and [mingw-w64](http://mingw-w64.yaxm.org/). At the moment (March 2015), both toolchains
are missing some declarations in header files, which will result in the following error message,
when running the ```go build```:

```
D:\projects.go\src\github.com\boombuler\hid> go build -v -work
WORK=C:\Users\xxx\AppData\Local\Temp\go-build011586055
github.com/boombuler/hid
# github.com/boombuler/hid
could not determine kind of name for C.HidD_FreePreparsedData
could not determine kind of name for C.HidD_GetPreparsedData
```

The solutions is simple: just add these four lines to your gcc toolchain header file ```hidsdi.h```
````C
/* http://msdn.microsoft.com/en-us/library/windows/hardware/ff538893(v=vs.85).aspx */
HIDAPI BOOLEAN NTAPI HidD_FreePreparsedData(PHIDP_PREPARSED_DATA PreparsedData);

/* http://msdn.microsoft.com/en-us/library/windows/hardware/ff539679(v=vs.85).aspx */
HIDAPI BOOLEAN NTAPI HidD_GetPreparsedData(HANDLE HidDeviceObject, PHIDP_PREPARSED_DATA *PreparsedData);
````
Depending on your gcc toolchain installation folder, the files are located in

``` C:\TDM-GCC-64\x86_64-w64-mingw32\include\hidsdi.h ```

or

``` c:\mingw-w64\x86_64-4.9.2-win32-seh-rt_v3-rev1\mingw64\x86_64-w64-mingw32\include\hidsdi.h ```

After patching the header file, this package will compile.
Future releases of the gcc toolchains will surely fix this issue.

## License

[![License: MIT](https://img.shields.io/:license-MIT-blue.svg)](http://opensource.org/licenses/MIT)
