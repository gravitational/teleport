package utils

import "os"

// IsDir is a helper function to quickly check if a given path is a valid directory
func IsDir(dirPath string) bool {
	fi, err := os.Stat(dirPath)
	if err == nil {
		return fi.IsDir()
	}
	return false
}
