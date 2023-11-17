//go:build linux

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

package native

import (
	"errors"
	"fmt"
	"io/fs"
	"os/user"
	"testing"

	"github.com/google/go-cmp/cmp"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/lib/linux"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestCollectDeviceData_linux(t *testing.T) {
	// Silence logging for tests.
	log.SetLevel(log.PanicLevel)

	u, err := user.Current()
	require.NoError(t, err, "reading current user")

	wantCD := &devicepb.DeviceCollectedData{
		CollectTime:           nil, // Verified by test body.
		OsType:                devicepb.OSType_OS_TYPE_LINUX,
		SerialNumber:          "PF0A0AAA",
		ModelIdentifier:       "21J50013US",
		OsVersion:             "22.04",
		OsBuild:               "22.04.3 LTS (Jammy Jellyfish)",
		OsUsername:            u.Name,
		ReportedAssetTag:      "No Asset Information",
		SystemSerialNumber:    "PF0A0AAA",
		BaseBoardSerialNumber: "L1AA00A00A0",
		OsId:                  "ubuntu",
	}

	dmiInfoSuccess := func() (*linux.DMIInfo, error) {
		return &linux.DMIInfo{
			ProductName:     wantCD.ModelIdentifier,
			ProductSerial:   wantCD.SystemSerialNumber,
			BoardSerial:     wantCD.BaseBoardSerialNumber,
			ChassisAssetTag: wantCD.ReportedAssetTag,
		}, nil
	}
	dmiInfoPermissionError := func() (*linux.DMIInfo, error) {
		return nil, fmt.Errorf("read DMI files: %w", fs.ErrPermission)
	}
	dmiInfoCacheNotFound := func() (*linux.DMIInfo, error) {
		return nil, errors.New("no cached DMI info")
	}

	// Default configuration reflects a successful DMI read with an empty cache.
	cddFuncs.parseOSRelease = func() (*linux.OSRelease, error) {
		return &linux.OSRelease{
			VersionID: wantCD.OsVersion,
			Version:   wantCD.OsBuild,
			ID:        wantCD.OsId,
		}, nil
	}
	cddFuncs.dmiInfoFromSysfs = dmiInfoSuccess
	cddFuncs.readDMIInfoCached = dmiInfoCacheNotFound
	cddFuncs.readDMIInfoEscalated = func() (*linux.DMIInfo, error) {
		return nil, errors.New("not implemented")
	}
	cddFuncs.saveDMIInfoToCache = func(d *linux.DMIInfo) error {
		// Failures here shouldn't make a difference.
		return errors.New("not implemented")
	}

	tests := []struct {
		name                 string
		mode                 CollectDataMode
		dmiFromSysfsOverride func() (*linux.DMIInfo, error)
		dmiFromCacheOverride func() (*linux.DMIInfo, error)
		dmiEscalatedOverride func() (*linux.DMIInfo, error)
		want                 *devicepb.DeviceCollectedData
	}{
		{
			name: "success without escalation",
			mode: CollectedDataAlwaysEscalate,
			want: wantCD,
		},
		{
			name:                 "AlwaysEscalate - success with escalation",
			mode:                 CollectedDataAlwaysEscalate,
			dmiFromSysfsOverride: dmiInfoPermissionError,
			dmiFromCacheOverride: func() (*linux.DMIInfo, error) {
				panic("cache lookup not allowed for this scenario")
			},
			dmiEscalatedOverride: dmiInfoSuccess,
			want:                 wantCD,
		},
		{
			name:                 "MaybeEscalate - success with cache",
			mode:                 CollectedDataMaybeEscalate,
			dmiFromSysfsOverride: dmiInfoPermissionError,
			dmiFromCacheOverride: dmiInfoSuccess,
			dmiEscalatedOverride: func() (*linux.DMIInfo, error) {
				panic("escalation not necessary for this scenario")
			},
			want: wantCD,
		},
		{
			name:                 "MaybeEscalate - success with escalation",
			mode:                 CollectedDataMaybeEscalate,
			dmiFromSysfsOverride: dmiInfoPermissionError,
			dmiFromCacheOverride: dmiInfoCacheNotFound,
			dmiEscalatedOverride: dmiInfoSuccess,
			want:                 wantCD,
		},
		{
			name:                 "NeverEscalate - success with cache",
			mode:                 CollectedDataNeverEscalate,
			dmiFromSysfsOverride: dmiInfoPermissionError,
			dmiFromCacheOverride: dmiInfoSuccess,
			dmiEscalatedOverride: func() (*linux.DMIInfo, error) {
				panic("escalation not allowed for this scenario")
			},
			want: wantCD,
		},
		{
			name: "NeverEscalate - returns what it can",
			mode: CollectedDataNeverEscalate,
			dmiFromSysfsOverride: func() (*linux.DMIInfo, error) {
				return &linux.DMIInfo{
					ProductName: wantCD.ModelIdentifier,
				}, fs.ErrPermission
			},
			dmiFromCacheOverride: dmiInfoCacheNotFound,
			dmiEscalatedOverride: func() (*linux.DMIInfo, error) {
				panic("escalation not allowed for this scenario")
			},
			want: func() *devicepb.DeviceCollectedData {
				cp := proto.Clone(wantCD).(*devicepb.DeviceCollectedData)
				cp.SerialNumber = ""
				cp.ReportedAssetTag = ""
				cp.SystemSerialNumber = ""
				cp.BaseBoardSerialNumber = ""
				return cp
			}(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Reset hooks after test.
			dmiFromSysfsBefore := cddFuncs.dmiInfoFromSysfs
			dmiFromCacheBefore := cddFuncs.readDMIInfoCached
			dmiEscalatedBefore := cddFuncs.readDMIInfoEscalated
			defer func() {
				cddFuncs.dmiInfoFromSysfs = dmiFromSysfsBefore
				cddFuncs.readDMIInfoCached = dmiFromCacheBefore
				cddFuncs.readDMIInfoEscalated = dmiEscalatedBefore
			}()

			// Set overrides.
			if test.dmiFromSysfsOverride != nil {
				cddFuncs.dmiInfoFromSysfs = test.dmiFromSysfsOverride
			}
			if test.dmiFromCacheOverride != nil {
				cddFuncs.readDMIInfoCached = test.dmiFromCacheOverride
			}
			if test.dmiEscalatedOverride != nil {
				cddFuncs.readDMIInfoEscalated = test.dmiEscalatedOverride
			}

			got, err := CollectDeviceData(test.mode)
			require.NoError(t, err, "CollectDeviceData")
			assert.NotNil(t, got.CollectTime, "CollectTime must not be nil")

			want := test.want
			want.CollectTime = got.CollectTime
			if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
				t.Errorf("CollectDeviceData mismatch (-want +got)\n%s", diff)
			}
		})
	}
}
