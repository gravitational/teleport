/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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

	// Read the various files concurrently.
	names := []string{
		"product_name",
		"product_serial",
		"board_serial",
		"chassis_asset_tag",
	}
	vals := make([]string, len(names))
	errs := make([]error, len(names))
	for i, name := range names {
		i := i
		name := name

		wg.Add(1)
		go func() {
			defer wg.Done()

			f, err := dmifs.Open(name)
			if err != nil {
				errs[i] = err
				return
			}
			defer f.Close()

			val, err := io.ReadAll(f)
			if err != nil {
				errs[i] = err
				return
			}

			vals[i] = strings.TrimSpace(string(val))
		}()
	}

	wg.Wait()

	return &DMIInfo{
		ProductName:     vals[0],
		ProductSerial:   vals[1],
		BoardSerial:     vals[2],
		ChassisAssetTag: vals[3],
	}, errors.Join(errs...)
}
