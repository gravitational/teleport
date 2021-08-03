//+build desktop_access_beta

package rdpclient

/*
// C proxy function trick, to allow calling Go callbacks from Rust.
// See https://github.com/golang/go/wiki/cgo#function-pointer-callbacks
#include <librdprs.h>
#include <stdint.h>

extern CGOError handleBitmapJump(int64_t, struct CGOBitmap);

CGOError handleBitmap_cgo(int64_t cp, struct CGOBitmap cb) {
	return handleBitmapJump(cp, cb);
}
*/
import "C"
