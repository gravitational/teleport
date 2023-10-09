//go:build !linux
// +build !linux

/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
Package uacc concerns itself with updating the user account database and log on nodes
that a client connects to with an interactive session.

This is a stub version that doesn't do anything and exists purely for compatibility purposes with systems we don't support.

We do not support macOS yet because they introduced ASL for user accounting with Mac OS X 10.6 (Snow Leopard)
and integrating with that takes additional effort.
*/
package uacc

import (
	"os"
)

// Open is a stub function.
func Open(utmpPath, wtmpPath string, username, hostname string, remote [4]int32, tty *os.File) error {
	return nil
}

// Close is a stub function.
func Close(utmpPath, wtmpPath string, tty *os.File) error {
	return nil
}

// UserWithPtyInDatabase is a stub function.
func UserWithPtyInDatabase(utmpPath string, username string) error {
	return nil
}

// LogFailedLogin is a stub function.
func LogFailedLogin(btmpPath, username, hostname string, remote [4]int32) error {
	return nil
}
