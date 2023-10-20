/*
Copyright 2022 Gravitational, Inc.

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

package protocol

import (
	"io"

	mssql "github.com/microsoft/go-mssqldb"
)

func readUcs2(r io.Reader, numchars int) (string, error) {
	buf := make([]byte, numchars)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return "", err
	}
	return mssql.ParseUCS2String(buf)
}
