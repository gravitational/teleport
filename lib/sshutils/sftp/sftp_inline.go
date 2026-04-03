package sftp

import (
	"github.com/pkg/sftp"

	"github.com/gravitational/teleport/session/sftputils"
)

const (
	//go:fix inline
	MethodGet = sftputils.MethodGet
	//go:fix inline
	MethodPut = sftputils.MethodPut
	//go:fix inline
	MethodOpen = sftputils.MethodOpen
	//go:fix inline
	MethodSetStat = sftputils.MethodSetStat
	//go:fix inline
	MethodRename = sftputils.MethodRename
	//go:fix inline
	MethodRmdir = sftputils.MethodRmdir
	//go:fix inline
	MethodMkdir = sftputils.MethodMkdir
	//go:fix inline
	MethodLink = sftputils.MethodLink
	//go:fix inline
	MethodSymlink = sftputils.MethodSymlink
	//go:fix inline
	MethodRemove = sftputils.MethodRemove
	//go:fix inline
	MethodList = sftputils.MethodList
	//go:fix inline
	MethodStat = sftputils.MethodStat
	//go:fix inline
	MethodLstat = sftputils.MethodLstat
	//go:fix inline
	MethodReadlink = sftputils.MethodReadlink
)

//go:fix inline
type File = sftputils.File

//go:fix inline
type FileSystem = sftputils.FileSystem

//go:fix inline
type PathExpansionError = sftputils.PathExpansionError

//go:fix inline
func ExpandHomeDir(pathStr string) (string, error) {
	return sftputils.ExpandHomeDir(pathStr)
}

//go:fix inline
type NonRecursiveDirectoryTransferError = sftputils.NonRecursiveDirectoryTransferError

//go:fix inline
func HandleFilecmd(req *sftp.Request, filesys sftputils.FileSystem) error {
	return sftputils.HandleFilecmd(req, filesys)
}

//go:fix inline
func HandleFilelist(req *sftp.Request, filesys sftputils.FileSystem) (sftp.ListerAt, error) {
	return sftputils.HandleFilelist(req, filesys)
}
