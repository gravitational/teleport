//go:build windows

package autoupdate

//go:generate go run golang.org/x/sys/windows/mkwinsyscall -output zsyscall_windows.go syscall_windows.go

//sys CryptMsgGetParam(msg windows.Handle, paramType uint32, index uint32, data *byte, dataLen *uint32) (err error) [failretval==0] = crypt32.CryptMsgGetParam
//sys CryptMsgClose(msg windows.Handle) (err error) [failretval==0] = crypt32.CryptMsgClose
//sys CertCompareCertificateName(encodingType uint32, name1 *windows.CertNameBlob, name2 *windows.CertNameBlob) (ok bool) = crypt32.CertCompareCertificateName
