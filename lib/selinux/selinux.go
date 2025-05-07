package selinux

// #cgo CFLAGS: -D_GNU_SOURCE -DDISABLE_RPM -DDISABLE_SETRANS -DNO_ANDROID_BACKEND -DNO_X_BACKEND -D_LARGEFILE_SOURCE -D_FILE_OFFSET_BITS=64 -DUSE_PCRE2 -DPCRE2_CODE_UNIT_WIDTH=8 -DHAVE_REALLOCARRAY
// #cgo pkg-config: libpcre2-8
// #include <selinux/selinux.h>
import "C"

func IsSelinuxEnabled() bool {
	return C.is_selinux_enabled() != 0
}
