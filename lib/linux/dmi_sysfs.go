// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package linux

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"
	"sync"
)

// DMIInfo holds information acquired from the device's DMI.
type DMIInfo struct {
	// ProductName of the device, as read from /sys/class/dmi/id/product_name.
	// Eg: "21J50013US".
	ProductName string

	// ProductSerial of the device, as read from /sys/class/dmi/id/product_serial.
	// Eg: "PF0A0AAA".
	ProductSerial string

	// BoardSerial of the device, as read from /sys/class/dmi/id/board_serial.
	// Eg: "L1AA00A00A0".
	BoardSerial string

	// ChassisAssetTag of the device, as read from
	// /sys/class/dmi/id/chassis_asset_tag.
	//
	// May contain a variety of strings to denote an unset asset tag, such as
	// "No Asset Information", "Default string", etc (creativity is the limit,
	// really).
	//
	// Eg: "No Asset Information".
	ChassisAssetTag string
}

// DMIInfoFromSysfs reads DMI info from /sys/class/dmi/id/.
//
// The method reads as much information as possible, so it always returns a
// non-nil [DMIInfo], even if it errors.
func DMIInfoFromSysfs() (*DMIInfo, error) {
	return DMIInfoFromFS(os.DirFS("/sys/class/dmi/id"))
}

// DMIInfoFromFS reads DMI from dmifs as if it was rooted at /sys/class/dmi/id/.
//
// The method reads as much information as possible, so it always returns a
// non-nil [DMIInfo], even if it errors.
func DMIInfoFromFS(dmifs fs.FS) (*DMIInfo, error) {
	var wg sync.WaitGroup

	var mu sync.Mutex // guards errs
	var errs []error
	appendErr := func(err error) {
		mu.Lock()
		errs = append(errs, err)
		mu.Unlock()
	}

	// Read the various files concurrently.
	var productName, productSerial, boardSerial, chassisAssetTag string
	for _, spec := range []struct {
		filename string
		out      *string
	}{
		{filename: "product_name", out: &productName},
		{filename: "product_serial", out: &productSerial},
		{filename: "board_serial", out: &boardSerial},
		{filename: "chassis_asset_tag", out: &chassisAssetTag},
	} {
		spec := spec

		wg.Add(1)
		go func() {
			defer wg.Done()

			f, err := dmifs.Open(spec.filename)
			if err != nil {
				appendErr(err)
				return
			}
			defer f.Close()

			val, err := io.ReadAll(f)
			if err != nil {
				appendErr(err)
				return
			}

			*spec.out = strings.TrimSpace(string(val))
		}()
	}

	wg.Wait()

	return &DMIInfo{
		ProductName:     productName,
		ProductSerial:   productSerial,
		BoardSerial:     boardSerial,
		ChassisAssetTag: chassisAssetTag,
	}, errors.Join(errs...)
}
