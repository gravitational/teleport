package devicetrustv1_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

type syncInventoryTest struct {
	name string

	start       *devicepb.SyncInventoryStart
	devicePages [][]*devicepb.Device

	wantCodes         [][]codes.Code // expected response codes, per page
	wantUniqueDevices int            // total unique IDs in the response

	// assertSync performs additional checks after all other assertions pass.
	assertSync func(t *testing.T, got [][]*devicepb.DeviceOrStatus, devsBefore []*devicepb.Device)
}

func TestService_SyncInventory(t *testing.T) {
	env := testenv.NewUsingT(t)

	ctx := context.Background()
	devices := env.DevicesClient

	jamfSource := devicepb.DeviceSource_builder{
		Name:   "jamf",
		Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF,
	}.Build()

	oldProfile := devicepb.DeviceProfile_builder{
		ModelIdentifier:   "MacBookPro9,2",
		OsVersion:         "13.2.1",
		OsBuild:           "22D68",
		OsUsernames:       []string{"admin", "llama"},
		JamfBinaryVersion: "10.45.0-t1678116779",
	}.Build()
	newProfile := devicepb.DeviceProfile_builder{
		ModelIdentifier:   oldProfile.GetModelIdentifier(),
		OsVersion:         "13.3.1",     // newer
		OsBuild:           "22E772610a", // newer
		OsUsernames:       oldProfile.GetOsUsernames(),
		JamfBinaryVersion: oldProfile.GetJamfBinaryVersion(),
	}.Build()

	// allDevices is the canonical set of devices for most tests.
	// A few test cases will add additional devices, but subsequent cases will
	// often restore storage to `allDevices`.
	allDevices := []*devicepb.Device{
		devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "llama",
			// Complete profile.
			Profile: oldProfile,
		}.Build(),
		devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "alpaca",
			// Profile omitted from here onwards for brevity.
		}.Build(),
		devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "ignoresource",
			Source: devicepb.DeviceSource_builder{
				Name:   "ignored",
				Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_INTUNE, // Sync source takes precedence.
			}.Build(),
		}.Build(),
		devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "dev1",
		}.Build(),
		devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "dev2",
		}.Build(),
		devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "dev3",
		}.Build(),
		devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
			AssetTag: "dev1", // same tag, different OS
		}.Build(),
	}
	llamaDev := allDevices[0]
	alpacaDev := allDevices[1]
	// ignoresource skipped
	dev1 := allDevices[3]
	dev2 := allDevices[4]
	dev3 := allDevices[5]
	// dev1/Windows skipped

	// Codes default to OK.
	allDevicesOK := make([]codes.Code, len(allDevices))

	assertNoChanges := func(t *testing.T, _ [][]*devicepb.DeviceOrStatus, devsBefore []*devicepb.Device) {
		t.Helper()

		got, err := listAllDevices(ctx, devices)
		if err != nil {
			t.Fatalf("listAllDevices failed: %v", err)
		}
		if diff := cmp.Diff(devsBefore, got, protocmp.Transform()); diff != "" {
			t.Errorf("Stored devices mismatch (-want +got)\n%s", diff)
		}
	}

	// assertStoredTags matches stored devices against `want`, considering only
	// the OsType and AssetTag fields.
	assertStoredDevs := func(wantDevs ...*devicepb.Device) func(t *testing.T, _ [][]*devicepb.DeviceOrStatus, _ []*devicepb.Device) {
		return func(t *testing.T, _ [][]*devicepb.DeviceOrStatus, _ []*devicepb.Device) {
			t.Helper()

			stored, err := listAllDevices(ctx, devices)
			if err != nil {
				t.Fatalf("listAllDevices failed: %v", err)
			}

			type deviceKey struct {
				OsType   devicepb.OSType
				AssetTag string
			}

			toDeviceKeys := func(devs []*devicepb.Device) map[deviceKey]struct{} {
				res := make(map[deviceKey]struct{})
				for _, d := range devs {
					res[deviceKey{
						OsType:   d.GetOsType(),
						AssetTag: d.GetAssetTag(),
					}] = struct{}{}
				}
				return res
			}

			got := toDeviceKeys(stored)
			want := toDeviceKeys(wantDevs)
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("Stored devices mismatch (-want +got)\n%s", diff)
			}
		}
	}

	// Run sync tests against `allDevices`.
	// The first test case creates `allDevices`, subsequent tests make no changes.
	runSyncInventoryTests(t, ctx, devices, []syncInventoryTest{
		{
			name: "partial sync",
			start: devicepb.SyncInventoryStart_builder{
				Source: jamfSource,
			}.Build(),
			devicePages: [][]*devicepb.Device{
				allDevices[0:2],
				allDevices[2:4],
				allDevices[4:],
			},
			wantCodes: [][]codes.Code{
				{
					codes.OK,
					codes.OK,
				},
				{
					codes.OK,
					codes.OK,
				},
				{
					codes.OK,
					codes.OK,
					codes.OK,
				},
			},
			wantUniqueDevices: len(allDevices),
		},
		{
			name: "partial no changes",
			start: devicepb.SyncInventoryStart_builder{
				Source: jamfSource,
			}.Build(),
			devicePages: [][]*devicepb.Device{
				allDevices,
			},
			wantCodes: [][]codes.Code{
				allDevicesOK,
			},
			wantUniqueDevices: len(allDevices),
			assertSync:        assertNoChanges,
		},
		{
			name: "full multiple pages",
			start: devicepb.SyncInventoryStart_builder{
				Source:              jamfSource,
				TrackMissingDevices: true,
			}.Build(),
			devicePages: [][]*devicepb.Device{
				allDevices[0:3],
				allDevices[3:6],
				allDevices[6:],
			},
			wantCodes: [][]codes.Code{
				{
					codes.OK,
					codes.OK,
					codes.OK,
				},
				{
					codes.OK,
					codes.OK,
					codes.OK,
				},
				{
					codes.OK,
				},
			},
			wantUniqueDevices: len(allDevices),
			assertSync:        assertNoChanges,
		},
		{
			name: "full single page",
			start: devicepb.SyncInventoryStart_builder{
				Source:              jamfSource,
				TrackMissingDevices: true,
			}.Build(),
			devicePages: [][]*devicepb.Device{
				allDevices,
			},
			wantCodes: [][]codes.Code{
				allDevicesOK,
			},
			wantUniqueDevices: len(allDevices),
			assertSync:        assertNoChanges,
		},
		{
			name: "empty full sync causes no deletions",
			start: devicepb.SyncInventoryStart_builder{
				Source:              jamfSource,
				TrackMissingDevices: true,
			}.Build(),
			devicePages: [][]*devicepb.Device{
				{}, // empty page
				{
					{OsType: 0, AssetTag: "missingOsType"},                // invalid
					{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: ""}, // invalid
				},
			},
			wantCodes: [][]codes.Code{
				{}, // empty first page
				{codes.InvalidArgument, codes.InvalidArgument},
			},
			wantUniqueDevices: 0, // nothing generated an Id
			assertSync:        assertNoChanges,
		},
	})

	// Test updates against `allDevices`.
	runSyncInventoryTests(t, ctx, devices, []syncInventoryTest{
		{
			name: "partial with update",
			start: devicepb.SyncInventoryStart_builder{
				Source: jamfSource,
			}.Build(),
			devicePages: [][]*devicepb.Device{
				{
					alpacaDev,
				},
				{
					dev1,
					dev1, // repeated
					dev2,
					{OsType: 0, AssetTag: "missingOsType"}, // invalid
					{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: ""}, // invalid
				},
				{
					dev3,
					{
						OsType:   llamaDev.GetOsType(),
						AssetTag: llamaDev.GetAssetTag(),
						Profile:  newProfile, // changed
					},
				},
			},
			wantCodes: [][]codes.Code{
				{
					codes.OK,
				},
				{
					codes.OK,
					codes.OK,
					codes.OK,
					codes.InvalidArgument,
					codes.InvalidArgument,
				},
				{
					codes.OK,
					codes.OK, // llama update
				},
			},
			wantUniqueDevices: 5,
		},
		{
			name: "full with update",
			start: devicepb.SyncInventoryStart_builder{
				Source: jamfSource,
			}.Build(),
			devicePages: [][]*devicepb.Device{
				{
					dev1,
					{
						OsType:   alpacaDev.GetOsType(),
						AssetTag: alpacaDev.GetAssetTag(),
						Profile:  newProfile, // added
					},
					dev2,
					dev3,
				},
			},
			wantCodes: [][]codes.Code{
				{
					codes.OK,
					codes.OK, // alpaca update
					codes.OK,
					codes.OK,
				},
			},
			wantUniqueDevices: 4,
		},
	})

	// Add new devices.
	runSyncInventoryTests(t, ctx, devices, []syncInventoryTest{
		{
			name: "partial with update and create",
			start: devicepb.SyncInventoryStart_builder{
				Source: jamfSource,
			}.Build(),
			devicePages: [][]*devicepb.Device{
				{
					dev1,
					dev2,
					{
						OsType:   devicepb.OSType_OS_TYPE_MACOS,
						AssetTag: "dev40",
					},
					{
						OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
						AssetTag: "dev40",
					},
				},
			},
			wantCodes: [][]codes.Code{
				{
					codes.OK,
					codes.OK,
					codes.OK,
					codes.OK,
				},
			},
			wantUniqueDevices: 4,
		},
	})

	// Destructive FULL updates.
	dev50 := devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_MACOS,
		AssetTag: "dev50",
	}.Build()
	dev50Win := devicepb.Device_builder{
		OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
		AssetTag: "dev50",
	}.Build()
	runSyncInventoryTests(t, ctx, devices, []syncInventoryTest{
		{
			name: "full with deletion",
			start: devicepb.SyncInventoryStart_builder{
				Source:              jamfSource,
				TrackMissingDevices: true,
			}.Build(),
			devicePages: [][]*devicepb.Device{
				{
					dev2,
					dev3,
					llamaDev,
					alpacaDev,
				},
				{
					dev50,    // new
					dev50Win, // new
				},
			},
			wantCodes: [][]codes.Code{
				{
					codes.OK, // noop
					codes.OK, // noop
					codes.OK, // noop
					codes.OK, // noop
				},
				{
					codes.OK, // created
					codes.OK, // created
				},
				{
					codes.OK, // dev1, deleted
					codes.OK, // ignoresource, deleted
					codes.OK, // dev1/Windows, deleted
					codes.OK, // dev40 deleted
					codes.OK, // dev40/Windows, deleted
				},
			},
			wantUniqueDevices: 11,
			assertSync:        assertStoredDevs(dev2, dev3, llamaDev, alpacaDev, dev50, dev50Win),
		},
		{
			name: "full with inventory wipe",
			start: devicepb.SyncInventoryStart_builder{
				Source:              jamfSource,
				TrackMissingDevices: true,
			}.Build(),
			devicePages: [][]*devicepb.Device{
				{dev2},
			},
			wantCodes: [][]codes.Code{
				{codes.OK}, // dev2, kept
				{
					codes.OK, // dev3, deleted
					codes.OK, // alpaca, deleted
					codes.OK, // llama, deleted
					codes.OK, // dev50, deleted
					codes.OK, // dev50/Windows, deleted
				},
			},
			wantUniqueDevices: 6,
			assertSync:        assertStoredDevs(dev2),
		},
		{
			name: "tracks devices that failed to update",
			start: devicepb.SyncInventoryStart_builder{
				Source:              jamfSource,
				TrackMissingDevices: true,
			}.Build(),
			devicePages: [][]*devicepb.Device{
				{
					{
						OsType:   dev2.GetOsType(),
						AssetTag: dev2.GetAssetTag(),
						Profile: devicepb.DeviceProfile_builder{
							OsUsernames: []string{"" /* invalid */, "llama"},
						}.Build(),
					},
				},
			},
			wantCodes: [][]codes.Code{
				{codes.InvalidArgument},
			},
			wantUniqueDevices: 0,                      // non-OK status for dev2
			assertSync:        assertStoredDevs(dev2), // dev2 not deleted!
		},
		{
			name: "full rebuild",
			start: devicepb.SyncInventoryStart_builder{
				Source:              jamfSource,
				TrackMissingDevices: true,
			}.Build(),
			devicePages: [][]*devicepb.Device{
				allDevices,
			},
			wantCodes: [][]codes.Code{
				allDevicesOK,
			},
			wantUniqueDevices: len(allDevices),
			assertSync:        assertStoredDevs(allDevices...),
		},
	})

	// Deletions and multiple sources.
	tctlDev, err := devices.CreateDevice(ctx, devicepb.CreateDeviceRequest_builder{
		Device: devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "createdByTctl",
		}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("CreateDevice failed: %v", err)
	}
	otherSource := devicepb.DeviceSource_builder{
		Name:   "myscript",
		Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_API,
	}.Build()
	runSyncInventoryTests(t, ctx, devices, []syncInventoryTest{
		{
			name: "other source takes ownership",
			start: devicepb.SyncInventoryStart_builder{
				Source:              otherSource,
				TrackMissingDevices: true,
			}.Build(),
			devicePages: [][]*devicepb.Device{
				{
					dev1, // takes ownership
					dev2, // takes ownership
					dev3, // takes ownership
				},
			},
			wantCodes: [][]codes.Code{
				{
					codes.OK,
					codes.OK,
					codes.OK,
				},
			},
			wantUniqueDevices: 3,
			assertSync:        assertStoredDevs(append(allDevices, tctlDev)...),
		},
		{
			name: "tracking respects source",
			start: devicepb.SyncInventoryStart_builder{
				Source:              jamfSource,
				TrackMissingDevices: true,
			}.Build(),
			devicePages: [][]*devicepb.Device{
				{dev1}, // takes ownership again
			},
			wantCodes: [][]codes.Code{
				{codes.OK},
				{
					codes.OK, // llama, deleted
					codes.OK, // alpaca, deleted
					codes.OK, // ignoresource, deleted
					codes.OK, // dev1/Windows, deleted
				},
			},
			wantUniqueDevices: 5,
			assertSync: assertStoredDevs(
				dev1,       // owned by jamf
				dev2, dev3, // owned by otherSource
				tctlDev, // owned by "null" source
			),
		},
		{
			name: "full rebuild",
			start: devicepb.SyncInventoryStart_builder{
				Source:              jamfSource,
				TrackMissingDevices: true,
			}.Build(),
			devicePages: [][]*devicepb.Device{
				allDevices,
			},
			wantCodes: [][]codes.Code{
				allDevicesOK,
			},
			wantUniqueDevices: len(allDevices),
			assertSync:        assertStoredDevs(append(allDevices, tctlDev)...),
		},
	})
}

func runSyncInventoryTests(t *testing.T, ctx context.Context, devices devicepb.DeviceTrustServiceClient, tests []syncInventoryTest) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Fetch stored devices prior to sync.
			devsBefore, err := listAllDevices(ctx, devices)
			if err != nil {
				t.Fatalf("listAllDevices failed: %v", err)
			}

			// Sync!
			got, err := syncInventoryPages(ctx, devices, test.start, test.devicePages)
			if err != nil {
				t.Fatalf("SyncInventory failed: %v", err)
			}

			// Verify response codes.
			gotCodes := make([][]codes.Code, len(got))
			for i, page := range got {
				gotCodes[i] = make([]codes.Code, len(page))
				for j, s := range page {
					gotCodes[i][j] = codes.Code(s.GetStatus().GetCode())
				}
			}
			if diff := cmp.Diff(test.wantCodes, gotCodes); diff != "" {
				t.Fatalf("SyncInventory codes mismatch (-want +got)\n%s", diff)
			}

			// Verify that non-end pages are not deletions.
			l := len(got)
			for i, page := range got[:l-1] {
				for j, s := range page {
					if s.GetDeleted() {
						t.Errorf("got[%v][%v]: item in non-end page is a deletion: %v", i, j, s)
					}
				}
			}
			// End page is either all deletes or no deletes.
			var isDeleted *bool
			for i, s := range got[l-1] {
				if codes.Code(s.GetStatus().GetCode()) != codes.OK {
					continue
				}
				if isDeleted == nil {
					isDeleted = proto.Bool(s.GetDeleted())
					continue
				}
				if s.GetDeleted() != *isDeleted {
					t.Errorf("got[%v][%v]: end page has a mix of deletes and non-deletes: %v (want %v)", l-1, i, s, *isDeleted)
				}
			}

			// Verify response against storage.
			uniqueDevs := make(map[string]struct{})
			for i, page := range got {
				for j, s := range page {
					switch {
					// Skip non-OK codes.
					case codes.Code(s.GetStatus().GetCode()) != codes.OK:
						continue
					// All OK codes must have an ID.
					case s.GetId() == "":
						t.Errorf("got[%v][%v].Id is empty", i, j)
						continue
					}
					uniqueDevs[s.GetId()] = struct{}{}

					// Fetch stored device.
					got, err := devices.GetDevice(ctx, devicepb.GetDeviceRequest_builder{
						DeviceId: s.GetId(),
					}.Build())
					switch {
					case s.GetDeleted() && trace.IsNotFound(err):
						continue
					case s.GetDeleted():
						t.Errorf("got[%v][%v]: GetDevice returned unexpected err=%v, want not found", i, j, err)
						continue
					case err != nil:
						t.Errorf("got[%v][%v]: GetDevice failed: %v", i, j, err)
						continue
					}

					// Verify stored device against request.
					want := proto.Clone(test.devicePages[i][j]).(*devicepb.Device)
					want.SetId(s.GetId())                       // Acquired from DeviceOrStatus
					want.SetSource(test.start.GetSource())      // Copied from start.Source
					want.SetApiVersion(got.GetApiVersion())     // system-managed
					want.SetCreateTime(got.GetCreateTime())     // system-managed
					want.SetUpdateTime(got.GetUpdateTime())     // system-managed
					want.SetEnrollStatus(got.GetEnrollStatus()) // system-managed
					if want.HasProfile() {
						want.GetProfile().SetUpdateTime(got.GetProfile().GetUpdateTime()) // system-managed
					}
					if diff := cmp.Diff(want, got, protocmp.Transform()); diff != "" {
						t.Errorf("SyncInventory: stored device mismatch (-want +got)\n%s", diff)
					}
				}
			}

			// Verify that the number of unique IDs is as expected.
			if got, want := len(uniqueDevs), test.wantUniqueDevices; got != want {
				t.Errorf("SyncInventory: got %v unique devices in response, want %v", got, want)
			}

			if test.assertSync != nil {
				test.assertSync(t, got, devsBefore)
			}
		})
	}
}

func TestService_SyncInventory_audit(t *testing.T) {
	emitter := &eventstest.MockRecorderEmitter{}
	env := testenv.NewUsingT(t, testenv.WithEmitter(emitter))

	ctx := context.Background()
	devices := env.DevicesClient

	startTracking := devicepb.SyncInventoryStart_builder{
		Source: devicepb.DeviceSource_builder{
			Name:   "jamf",
			Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF,
		}.Build(),
		TrackMissingDevices: true,
	}.Build()

	mustSync := func(t *testing.T, devs []*devicepb.Device) {
		t.Helper()
		_, err := syncInventoryPages(ctx, devices, startTracking, [][]*devicepb.Device{devs})
		if err != nil {
			t.Fatalf("SyncInventory failed: %v", err)
		}
	}

	createEvent := wantEvent{Type: events.DeviceCreateEvent, Code: events.DeviceCreateCode}
	updateEvent := wantEvent{Type: events.DeviceUpdateEvent, Code: events.DeviceUpdateCode}
	deleteEvent := wantEvent{Type: events.DeviceDeleteEvent, Code: events.DeviceDeleteCode}

	// Create the initial batch of devices.
	dev1 := devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "dev1"}.Build()
	dev1Win := devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_WINDOWS, AssetTag: "dev1"}.Build()
	dev2 := devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "dev2"}.Build()
	dev3 := devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "dev3"}.Build()
	dev4 := devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "dev4"}.Build()
	mustSync(t, []*devicepb.Device{
		dev1,
		dev1Win,
		dev2,
		dev3,
		dev4,
	})
	assertEvents(t, emitter.Events(), []wantEvent{
		createEvent,
		createEvent,
		createEvent,
		createEvent,
		createEvent,
	})
	emitter.Reset()

	// Cause a few additional creations, updates and deletions.
	dev11 := devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "dev11"}.Build()
	dev12 := devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "dev12"}.Build()
	newProfile := devicepb.DeviceProfile_builder{
		OsVersion: "13.3.1",
		OsBuild:   "22E772610a",
	}.Build()
	mustSync(t, []*devicepb.Device{
		dev1Win, // noop
		dev2,    // noop
		devicepb.Device_builder{OsType: 0, AssetTag: "invalid1"}.Build(),
		devicepb.Device_builder{OsType: 0, AssetTag: "invalid2"}.Build(),
		dev11, // created
		dev12, // created
		devicepb.Device_builder{OsType: dev3.GetOsType(), AssetTag: dev3.GetAssetTag(), Profile: newProfile}.Build(), // updated
		// dev1 deleted
		// dev4 deleted
	})

	// Assert audit events.
	// Note that the order of audit events is not deterministic for this stream.
	got := emitter.Events()
	slices.SortFunc(got, func(a, b apievents.AuditEvent) int {
		return strings.Compare(a.GetType(), b.GetType())
	})
	assertEvents(t, got, []wantEvent{
		createEvent,
		createEvent,
		deleteEvent,
		deleteEvent,
		updateEvent,
	})

	// Assert that audit user was not modified to "Jamf Service" (calling identity
	// here doesn't match the prerequisites).
	if len(got) > 0 {
		deviceEvent := got[0].(*apievents.DeviceEvent2) // guaranteed by assertEvents.
		const prefix = "Jamf Service"
		if strings.HasPrefix(deviceEvent.User, prefix) {
			t.Errorf(`event.User=%q, want != "%s*"`, deviceEvent.User, prefix)
		}
	}
}

// TestService_SyncInventory_jamfServiceAudit tests that events from Jamf
// Service are attributed to the "Jamf Service" user.
func TestService_SyncInventory_jamfServiceUser(t *testing.T) {
	// Made-up but real-looking service serviceUser.
	const serviceUser = "5cb94254-ee92-4a99-b898-0dfa8d3be8aa.example.teleport.sh"

	authorizer := &jamfServiceAuthorizer{
		User: serviceUser,
	}
	emitter := &eventstest.MockRecorderEmitter{}
	env := testenv.NewUsingT(t,
		testenv.WithAuthorizer(authorizer),
		testenv.WithEmitter(emitter),
	)

	devicesClient := env.DevicesClient
	ctx := context.Background()

	// Sync a device so an event is issued.
	// We only care about the audit event, not the action taken to cause it.
	start := devicepb.SyncInventoryStart_builder{
		Source: devicepb.DeviceSource_builder{
			Name:   "jamf",
			Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF,
		}.Build(),
	}.Build()
	dev1 := devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "dev1"}.Build()
	if _, err := syncInventoryPages(
		ctx,
		devicesClient,
		start,
		[][]*devicepb.Device{{dev1}},
	); err != nil {
		t.Fatalf("SyncInventory failed: %v", err)
	}

	// Assert emitted events. This guarantees length/type/etc on success.
	gotEvents := emitter.Events()
	assertEvents(t, gotEvents, []wantEvent{
		{Type: events.DeviceCreateEvent, Code: events.DeviceCreateCode},
	})

	if len(gotEvents) > 0 {
		// We cannot override the "context user" returned by authz.UserFromContext/
		// authz.ClientUserMetadata, so the best we can do here is match by regex.
		wantRE := regexp.MustCompile(`^Jamf Service \(.+\)$`)
		for i, event := range gotEvents {
			deviceEvent := event.(*apievents.DeviceEvent2) // guaranteed by assertEvents.
			if got := deviceEvent.User; !wantRE.MatchString(got) {
				t.Errorf("gotEvents[%d].User=%q, want match against `%s`", i, got, wantRE.String())
			}
		}
	}
}

// jamfServiceAuthorizer mimics Jamf Service access by creating an authz.Context
// that passes an `authz.HasBuiltinRole(ctx, types.RoleMDM)` check.
type jamfServiceAuthorizer struct {
	User string
}

func (a *jamfServiceAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	u, err := types.NewUser(a.User)
	if err != nil {
		return nil, fmt.Errorf("new user: %w", err)
	}
	return &authz.Context{
		User:    u,
		Checker: &jamfServiceChecker{},
		Identity: authz.BuiltinRole{
			Role:     types.RoleMDM,
			Username: u.GetName(),
		},
		AdminActionAuthState: authz.AdminActionAuthNotRequired,
	}, nil
}

// jamfServiceChecker accompanies jamfServiceAuthorizer.
type jamfServiceChecker struct {
	testenv.NoopChecker
}

func (c *jamfServiceChecker) HasRole(role string) bool {
	return role == string(types.RoleMDM)
}

func TestService_SyncInventory_devicesToRemove(t *testing.T) {
	emitter := &eventstest.MockRecorderEmitter{}
	env := testenv.NewUsingT(t, testenv.WithEmitter(emitter))

	ctx := context.Background()
	devicesClient := env.DevicesClient

	source1 := devicepb.DeviceSource_builder{
		Name:   "jamf",
		Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF,
	}.Build()
	source2 := devicepb.DeviceSource_builder{
		Name:   "intune",
		Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_INTUNE,
	}.Build()

	allDevices := []*devicepb.Device{
		devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "llama"}.Build(),
		devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "alpaca"}.Build(),
		devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "dev1"}.Build(),
		devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_WINDOWS, AssetTag: "dev1"}.Build(), // different OS!
		devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "dev2"}.Build(),
		devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "dev3"}.Build(),
		devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_WINDOWS, AssetTag: "dev4"}.Build(),
		devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_WINDOWS, AssetTag: "dev5"}.Build(),
	}
	llama := allDevices[0]
	alpaca := allDevices[1]
	dev1 := allDevices[2]
	dev1Windows := allDevices[3]
	dev2 := allDevices[4]
	dev3 := allDevices[5]
	dev4 := allDevices[6]
	dev5 := allDevices[7]

	// Sync a few devices with source1 and others with source2.
	syncResp, err := syncInventoryPages(
		ctx,
		devicesClient,
		devicepb.SyncInventoryStart_builder{Source: source1}.Build(),
		[][]*devicepb.Device{allDevices})
	if err != nil {
		t.Fatalf("syncInventoryPages failed: %v", err)
	}

	// Assign IDs back to the original devices.
	for i, s := range syncResp[0] {
		// Sanity checks, this is covered by other tests.
		switch {
		case codes.Code(s.GetStatus().GetCode()) != codes.OK:
			t.Fatalf("Device %v has non-OK code: %s", i, codes.Code(s.GetStatus().GetCode()))
		case s.GetDeleted():
			t.Fatalf("Device %v has Deleted set: %#v", i, s)
		case s.GetId() == "":
			t.Fatalf("Device %v has an empty Id: %#v", i, s)
		}
		allDevices[i].SetId(s.GetId())
	}

	// "Transfer" a few devices to source2.
	if _, err := syncInventoryPages(
		ctx,
		devicesClient,
		devicepb.SyncInventoryStart_builder{Source: source2}.Build(),
		[][]*devicepb.Device{{dev4, dev5}}); err != nil {
		t.Fatalf("syncInventoryPages failed: %v", err)
	}

	// Test proper begins here.
	devsToUpsert := []*devicepb.Device{dev4}
	devsToRemove := []*devicepb.Device{
		nil, // NOK
		{},  // NOK, lacks identifiers
		devicepb.Device_builder{Id: llama.GetId(), OsType: llama.GetOsType(), AssetTag: "notllama"}.Build(),          // NOK, identifiers don't match
		devicepb.Device_builder{Id: llama.GetId(), OsType: llama.GetOsType(), AssetTag: llama.GetAssetTag()}.Build(), // OK, identifiers match
		devicepb.Device_builder{Id: dev2.GetId()}.Build(),
		devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_LINUX, AssetTag: dev3.GetAssetTag()}.Build(), // unknown device
		devicepb.Device_builder{OsType: devicepb.OSType_OS_TYPE_MACOS, AssetTag: "unknown"}.Build(),          // unknown device
		devicepb.Device_builder{Id: "unknown"}.Build(),
		devicepb.Device_builder{OsType: dev1.GetOsType(), AssetTag: dev1.GetAssetTag()}.Build(),
		dev4, // OK, source1 took ownership
		dev5, // NOK, source2 has ownership
	}
	wantOutcomes := []struct {
		id   string
		code codes.Code
		err  string
	}{
		{code: codes.InvalidArgument, err: "no identifiers"},
		{code: codes.InvalidArgument, err: "no identifiers"},
		{code: codes.InvalidArgument, err: "don't match"},
		{id: llama.GetId()},
		{id: dev2.GetId()},
		{code: codes.NotFound},
		{code: codes.NotFound},
		{code: codes.NotFound},
		{id: dev1.GetId()},
		{id: dev4.GetId()},
		{code: codes.InvalidArgument, err: "owned by another source"},
	}
	wantEvents := []wantEvent{
		{Type: events.DeviceUpdateEvent, Code: events.DeviceUpdateCode}, // dev4, devicesToUpsert
		{Type: events.DeviceDeleteEvent, Code: events.DeviceDeleteCode}, // llama
		{Type: events.DeviceDeleteEvent, Code: events.DeviceDeleteCode}, // dev2
		{Type: events.DeviceDeleteEvent, Code: events.DeviceDeleteCode}, // dev1
		{Type: events.DeviceDeleteEvent, Code: events.DeviceDeleteCode}, // dev4
	}

	// Run stream with deletions.
	emitter.Reset()
	deleteResp, err := syncInventoryDelete(ctx, devicesClient, devicepb.SyncInventoryStart_builder{Source: source1}.Build(), devsToUpsert, devsToRemove)
	if err != nil {
		t.Fatalf("SyncInventory deletion stream failed: %v", err)
	}

	// We expect 2 pages:
	// - page1 has the successful update of dev4
	// - page2 has the deletions
	switch {
	case len(deleteResp) != 2:
		t.Fatalf("SyncInventory returned %v pages, want 2", len(deleteResp))
	case len(deleteResp[0]) != 1:
		t.Fatalf("SyncInventory returned %v updated devices, want 1", len(deleteResp[0]))
	case codes.Code(deleteResp[0][0].GetStatus().GetCode()) != codes.OK:
		t.Fatalf("SyncInventory returned device update code=%s, want OK", codes.Code(deleteResp[0][0].GetStatus().GetCode()))
	case len(deleteResp[1]) != len(devsToRemove):
		t.Fatalf("SyncInventory returned %v deleted devices, want %v", len(deleteResp[1]), len(devsToRemove))
	}

	// Verify errors/successes on page2.
	for i, s := range deleteResp[1] {
		want := wantOutcomes[i]

		// Assert response code.
		gotCode := codes.Code(s.GetStatus().GetCode())
		if gotCode != want.code {
			t.Errorf("SyncInventory: deleted device #%v code=%s(%q), want %s", i, gotCode, s.GetStatus().GetMessage(), want.code)
		}

		// Assert failure message, it helps to distinguish errors.
		if got, want := s.GetStatus().GetMessage(), want.err; !strings.Contains(got, want) {
			t.Errorf("SyncInventory: deleted device #%v message=%q, want %q", i, got, want)
		}

		if gotCode != codes.OK {
			continue
		}
		if !s.GetDeleted() {
			t.Errorf("SyncInventory: deleted device #%v deleted=%v, want true", i, s.GetDeleted())
		}

		// Assert deletions.
		if _, err := devicesClient.GetDevice(ctx, devicepb.GetDeviceRequest_builder{DeviceId: s.GetId()}.Build()); !trace.IsNotFound(err) {
			t.Errorf("SyncInventory: querying deleted device #%v returned err=%v, want NotFound", i, err)
		}
	}

	// Assert non-deletions.
	for _, dev := range []*devicepb.Device{
		alpaca,      // delete not requested
		dev1Windows, // delete requested for dev1/macOS
		dev3,        // delete requested for wrong OsType
		dev5,        // owned by source2
	} {
		if _, err := devicesClient.GetDevice(ctx, devicepb.GetDeviceRequest_builder{DeviceId: dev.GetId()}.Build()); err != nil {
			t.Errorf("GetDevice(%s/%v) returned err=%v, want nil (wrongly deleted?)", dev.GetOsType(), dev.GetAssetTag(), err)
		}
	}

	// Assert audit events.
	assertEvents(t, emitter.Events(), wantEvents)
}

// TestService_SyncInventory_missingDevices tests missing devices aspects not
// covered by TestService_SyncInventory.
func TestService_SyncInventory_missingDevices(t *testing.T) {
	env := testenv.NewUsingT(t)

	devices := env.DevicesClient
	ctx := context.Background()

	source := devicepb.DeviceSource_builder{
		Name:   "jamf",
		Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF,
	}.Build()

	// Prepare test devices.
	var allDevs []*devicepb.Device
	for _, dev := range []*devicepb.Device{
		devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "llama",
			Source:   source,
			Profile: devicepb.DeviceProfile_builder{
				ExternalId: "1",
			}.Build(),
		}.Build(),
		devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "alpaca",
			Source:   source,
			Profile: devicepb.DeviceProfile_builder{
				ExternalId: "2",
			}.Build(),
		}.Build(),
		devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "camel",
			Source:   source,
			Profile: devicepb.DeviceProfile_builder{
				ExternalId: "3",
			}.Build(),
		}.Build(),
		devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_IPADOS,
			AssetTag: "ipad",
			Source:   source,
			Profile: devicepb.DeviceProfile_builder{
				ExternalId: "4",
			}.Build(),
		}.Build(),
		devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_WINDOWS,
			AssetTag: "win",
		}.Build(),
		devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_LINUX,
			AssetTag: "linux",
		}.Build(),
	} {
		created, err := devices.CreateDevice(ctx, devicepb.CreateDeviceRequest_builder{
			Device: dev,
		}.Build())
		if err != nil {
			t.Fatalf("CreateDevice failed: %v", err)
		}
		allDevs = append(allDevs, created)
	}
	llamaDev := allDevs[0]
	alpacaDev := allDevs[1]
	camelDev := allDevs[2]
	ipadDev := allDevs[3]

	// opts is used to compare Device slices.
	opts := []cmp.Option{
		cmpopts.SortSlices(func(d1, d2 *devicepb.Device) bool { return d1.GetId() < d2.GetId() }),
		protocmp.Transform(),
	}

	tests := []struct {
		name         string
		osTypes      []devicepb.OSType
		devsToUpsert []*devicepb.Device
		missingFn    func([]*devicepb.Device) []*devicepb.Device
		wantErr      string
		wantMissing  []*devicepb.Device
		wantStored   []*devicepb.Device
	}{
		{
			name:         "echoing non-missing device fails stream",
			devsToUpsert: []*devicepb.Device{llamaDev},
			missingFn: func(_ []*devicepb.Device) []*devicepb.Device {
				return []*devicepb.Device{llamaDev} // not missing!
			},
			// win and linux dev aren't owned by Jamf, so they aren't considered
			// missing. The iPad is owned by Jamf but its OsType is not in the
			// computer-types default, so it isn't reported either.
			wantMissing: []*devicepb.Device{alpacaDev, camelDev},
			wantErr:     "missing devices",
			wantStored:  allDevs,
		},
		{
			name:         "non-echoed devices are not deleted",
			devsToUpsert: []*devicepb.Device{alpacaDev},
			missingFn: func(_ []*devicepb.Device) []*devicepb.Device {
				return nil // nothing echoed, nothing removed
			},
			wantMissing: []*devicepb.Device{llamaDev, camelDev},
			wantStored:  allDevs,
		},
		{
			name:         "explicit os_types: macOS only",
			osTypes:      []devicepb.OSType{devicepb.OSType_OS_TYPE_MACOS},
			devsToUpsert: nil,
			missingFn: func(_ []*devicepb.Device) []*devicepb.Device {
				return nil // nothing echoed, nothing removed
			},
			// Same set as the empty-os_types default in this fixture: the iPad is
			// filtered out.
			wantMissing: []*devicepb.Device{llamaDev, alpacaDev, camelDev},
			wantStored:  allDevs,
		},
		{
			name:         "explicit os_types: macOS + iPadOS",
			osTypes:      []devicepb.OSType{devicepb.OSType_OS_TYPE_MACOS, devicepb.OSType_OS_TYPE_IPADOS},
			devsToUpsert: nil,
			missingFn: func(_ []*devicepb.Device) []*devicepb.Device {
				return nil // nothing echoed, nothing removed
			},
			// Opting in to iPadOS surfaces the iPad as missing.
			wantMissing: []*devicepb.Device{llamaDev, alpacaDev, camelDev, ipadDev},
			wantStored:  allDevs,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			missingFn := func(missing []*devicepb.Device) []*devicepb.Device {
				// Missing devices are only expected to have the fields below.
				want := make([]*devicepb.Device, len(test.wantMissing))
				for i, d := range test.wantMissing {
					want[i] = devicepb.Device_builder{
						Id:       d.GetId(),
						OsType:   d.GetOsType(),
						AssetTag: d.GetAssetTag(),
						Profile: devicepb.DeviceProfile_builder{
							ExternalId: d.GetProfile().GetExternalId(),
						}.Build(),
					}.Build()
				}
				// Assert missing devices.
				if diff := cmp.Diff(want, missing, opts...); diff != "" {
					t.Errorf("Missing devices mismatch (-want +got)\n%s", diff)
				}

				return test.missingFn(missing)
			}

			// Run SyncInventory stream.
			_, err := syncInventoryMissing(ctx, devices, source, test.osTypes, test.devsToUpsert, missingFn)
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr, "SyncInventory error mismatch")
				// Keep asserting statuses and storage.
			} else if err != nil {
				t.Fatalf("SyncInventory failed: %v", err)
			}

			// Assert stored.
			storedDevs, err := listAllDevices(ctx, devices)
			if err != nil {
				t.Fatalf("listAllDevices failed: %v", err)
			}
			if diff := cmp.Diff(test.wantStored, storedDevs, opts...); diff != "" {
				t.Errorf("Stored devices mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

// TestService_SyncInventory_osTypeFiltering verifies that SyncInventoryStart's
// os_types field filters upserts and removals: devices whose OsType is not in
// the declared os_types are rejected with a per-device error. Devices whose
// OsType is in the list proceed normally.
func TestService_SyncInventory_osTypeFiltering(t *testing.T) {
	source := devicepb.DeviceSource_builder{
		Name:   "jamf",
		Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF,
	}.Build()

	t.Run("upsert", func(t *testing.T) {
		env := testenv.NewUsingT(t)
		devices := env.DevicesClient

		macDev := devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "mac1",
		}.Build()
		iosDev := devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_IOS,
			AssetTag: "ios1",
		}.Build()
		ipadDev := devicepb.Device_builder{
			OsType:   devicepb.OSType_OS_TYPE_IPADOS,
			AssetTag: "ipad1",
		}.Build()

		runSyncInventoryTests(t, t.Context(), devices, []syncInventoryTest{
			{
				name: "rejects upserts whose os_type is not in os_types",
				start: devicepb.SyncInventoryStart_builder{
					Source:  source,
					OsTypes: []devicepb.OSType{devicepb.OSType_OS_TYPE_MACOS},
				}.Build(),
				devicePages: [][]*devicepb.Device{
					{macDev, iosDev, ipadDev},
				},
				wantCodes: [][]codes.Code{
					{codes.OK, codes.InvalidArgument, codes.InvalidArgument},
				},
				wantUniqueDevices: 1,
			},
			{
				name: "accepts upserts whose os_type is in os_types",
				start: devicepb.SyncInventoryStart_builder{
					Source:  source,
					OsTypes: []devicepb.OSType{devicepb.OSType_OS_TYPE_IOS, devicepb.OSType_OS_TYPE_IPADOS},
				}.Build(),
				devicePages: [][]*devicepb.Device{
					{ipadDev, iosDev},
				},
				wantCodes: [][]codes.Code{
					{codes.OK, codes.OK},
				},
				wantUniqueDevices: 2,
			},
			{
				// Empty os_types falls back to the computer OS types for
				// older clients: macOS accepted, iOS/iPadOS rejected.
				name: "empty os_types accepts only computer os_type values",
				start: devicepb.SyncInventoryStart_builder{
					Source: source,
				}.Build(),
				devicePages: [][]*devicepb.Device{
					{macDev, iosDev, ipadDev},
				},
				wantCodes: [][]codes.Code{
					{codes.OK, codes.InvalidArgument, codes.InvalidArgument},
				},
				wantUniqueDevices: 1,
			},
		})
	})

	t.Run("remove", func(t *testing.T) {
		env := testenv.NewUsingT(t)
		ctx := t.Context()
		devices := env.DevicesClient

		macStored, err := devices.CreateDevice(ctx, devicepb.CreateDeviceRequest_builder{
			Device: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_MACOS,
				AssetTag: "mac1",
				Source:   source,
				Profile:  devicepb.DeviceProfile_builder{ExternalId: "1"}.Build(),
			}.Build(),
		}.Build())
		if err != nil {
			t.Fatalf("CreateDevice macOS failed: %v", err)
		}
		iosStored, err := devices.CreateDevice(ctx, devicepb.CreateDeviceRequest_builder{
			Device: devicepb.Device_builder{
				OsType:   devicepb.OSType_OS_TYPE_IOS,
				AssetTag: "ios1",
				Source:   source,
				Profile:  devicepb.DeviceProfile_builder{ExternalId: "2"}.Build(),
			}.Build(),
		}.Build())
		if err != nil {
			t.Fatalf("CreateDevice iOS failed: %v", err)
		}

		// Start sync scoped to macOS devices.
		gotStatuses, err := syncInventoryDelete(ctx, devices,
			devicepb.SyncInventoryStart_builder{
				Source:  source,
				OsTypes: []devicepb.OSType{devicepb.OSType_OS_TYPE_MACOS},
			}.Build(),
			nil, /* devsToUpsert */
			[]*devicepb.Device{macStored, iosStored},
		)
		if err != nil {
			t.Fatalf("syncInventoryDelete failed: %v", err)
		}

		if len(gotStatuses) != 1 {
			t.Fatalf("syncInventoryDelete returned %d status pages, want 1", len(gotStatuses))
		}
		gotCodes := make([]codes.Code, len(gotStatuses[0]))
		for i, s := range gotStatuses[0] {
			gotCodes[i] = codes.Code(s.GetStatus().GetCode())
		}
		wantCodes := []codes.Code{codes.OK, codes.InvalidArgument}
		if diff := cmp.Diff(wantCodes, gotCodes); diff != "" {
			t.Errorf("Remove status codes mismatch (-want +got)\n%s", diff)
		}

		// The iOS device must still exist; the macOS one is gone.
		if _, err := devices.GetDevice(ctx, devicepb.GetDeviceRequest_builder{DeviceId: macStored.GetId()}.Build()); !trace.IsNotFound(err) {
			t.Errorf("macOS device unexpectedly survived removal: err=%v", err)
		}
		if _, err := devices.GetDevice(ctx, devicepb.GetDeviceRequest_builder{DeviceId: iosStored.GetId()}.Build()); err != nil {
			t.Errorf("iOS device unexpectedly gone after rejected removal: %v", err)
		}
	})
}

// TestService_SyncInventory_ignoreUsageBased verifies that legacy
// device trust limits from old licenses no longer apply.
// (see https://github.com/gravitational/teleport.e/issues/7490)
func TestService_SyncInventory_ignoreUsageBased(t *testing.T) {
	t.Parallel()

	env := testenv.NewUsingT(t, testenv.WithModules(&modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.DeviceTrust:            {Enabled: true},
				entitlements.MobileDeviceManagement: {Enabled: false},
			},
		},
	}))

	devices := env.DevicesClient
	ctx := context.Background()

	// Attempt to sync.
	_, err := syncInventoryPages(ctx, devices, devicepb.SyncInventoryStart_builder{
		Source: devicepb.DeviceSource_builder{
			Name:   "jamf",
			Origin: devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF,
		}.Build(),
	}.Build(), [][]*devicepb.Device{
		{{
			OsType:   devicepb.OSType_OS_TYPE_MACOS,
			AssetTag: "synced",
		}},
	})

	assert.NoError(t, err, "SyncInventory must work even if MobileDeviceManagement entitlement is disabled")
}

// syncInventoryPages sends `startReq`, then `devicePages` as `devices_to_add`
// (one send per page), and finally `endReq` to a SyncInventory stream.
//
// Statuses are captured, one per sent page, and returned.
//
// Any missing devices are echoed back for deletion. The deletion statuses are
// appended as a single status page in the end, when present.
func syncInventoryPages(
	ctx context.Context,
	devices devicepb.DeviceTrustServiceClient,
	startReq *devicepb.SyncInventoryStart,
	devicePages [][]*devicepb.Device,
) ([][]*devicepb.DeviceOrStatus, error) {
	stream, err := devices.SyncInventory(ctx)
	if err != nil {
		return nil, err
	}

	// Start sync.
	if err := stream.Send(devicepb.SyncInventoryRequest_builder{
		Start: proto.ValueOrDefault(startReq),
	}.Build()); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("start: Send: %w", err)
	}
	resp, err := stream.Recv()
	if err != nil {
		return nil, err // unwrapped for easier assertions
	}
	if resp.GetAck() == nil {
		return nil, fmt.Errorf("start: got payload=%T, want Ack", resp.GetPayload())
	}

	// Send device pages.
	results := make([][]*devicepb.DeviceOrStatus, 0, len(devicePages)+1)
	for _, page := range devicePages {
		if err := stream.Send(devicepb.SyncInventoryRequest_builder{
			DevicesToUpsert: devicepb.SyncInventoryDevices_builder{
				Devices: page,
			}.Build(),
		}.Build()); err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("devices: Send: %w", err)
		}
		resp, err = stream.Recv()
		if err != nil {
			return nil, fmt.Errorf("devices: Recv: %w", err)
		}
		res := resp.GetResult()
		if res == nil {
			return nil, fmt.Errorf("devices: got payload=%T, want Result", resp.GetPayload())
		}
		results = append(results, res.GetDevices())
	}

	// End sync.
	if err := stream.Send(devicepb.SyncInventoryRequest_builder{
		End: &devicepb.SyncInventoryEnd{},
	}.Build()); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("end: Send: %w", err)
	}

	// Handle missing devices.
	var deletes []*devicepb.DeviceOrStatus
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break // Server closed the stream.
		}
		if err != nil {
			return nil, fmt.Errorf("end: Recv: %w", err)
		}
		missingResp := resp.GetMissingDevices()
		if missingResp == nil {
			return nil, fmt.Errorf("end: got payload=%T, want MissingDevices", resp.GetPayload())
		}

		// Verify missing devices.
		missingDevs := missingResp.GetDevices()
		if len(missingDevs) == 0 {
			return nil, errors.New("got empty missing devices page")
		}
		for i, dev := range missingDevs {
			if dev.GetId() == "" ||
				dev.GetOsType() == devicepb.OSType_OS_TYPE_UNSPECIFIED ||
				dev.GetAssetTag() == "" {
				return nil, fmt.Errorf("missing device #%v missing required fields: %#v", i, dev)
			}
		}

		// Echo devices back for deletion.
		if err := stream.Send(devicepb.SyncInventoryRequest_builder{
			DevicesToRemove: devicepb.SyncInventoryDevices_builder{
				Devices: missingDevs,
			}.Build(),
		}.Build()); err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("end: Send devices_to_remove: %w", err)
		}

		// Receive and verify report.
		resp, err = stream.Recv()
		if err != nil {
			return nil, fmt.Errorf("err: Recv removed devices: %w", err)
		}
		resultResp := resp.GetResult()
		if resultResp == nil {
			return nil, fmt.Errorf("end: got payload=%T, want Result", resp.GetPayload())
		}
		resultDevs := resultResp.GetDevices()
		if got, want := len(resultDevs), len(missingDevs); got != want {
			return nil, fmt.Errorf("missing devices result has %v statuses, want %v", got, want)
		}
		deletes = append(deletes, resultDevs...)
	}
	if len(deletes) > 0 {
		results = append(results, deletes)
	}

	return results, nil
}

// syncInventoryDelete runs a delete-focused SyncInventory stream.
func syncInventoryDelete(
	ctx context.Context,
	devicesClient devicepb.DeviceTrustServiceClient,
	start *devicepb.SyncInventoryStart,
	devsToUpsert, devsToRemove []*devicepb.Device,
) ([][]*devicepb.DeviceOrStatus, error) {
	// Start stream.
	stream, err := devicesClient.SyncInventory(ctx)
	if err != nil {
		return nil, fmt.Errorf("init: %w", err)
	}
	if err := stream.Send(devicepb.SyncInventoryRequest_builder{
		Start: proto.ValueOrDefault(start),
	}.Build()); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("start Send: %w", err)
	}
	if _, err = stream.Recv(); err != nil {
		return nil, fmt.Errorf("start Recv: %w", err)
	}

	// Send DevicesToUpsert, if any.
	var statuses [][]*devicepb.DeviceOrStatus
	if len(devsToUpsert) > 0 {
		if err := stream.Send(devicepb.SyncInventoryRequest_builder{
			DevicesToUpsert: devicepb.SyncInventoryDevices_builder{Devices: devsToUpsert}.Build(),
		}.Build()); err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("devicesToUpsert Send: %w", err)
		}
		resp, err := stream.Recv()
		if err != nil {
			return nil, fmt.Errorf("devicesToUpsert Recv: %w", err)
		}
		statuses = append(statuses, resp.GetResult().GetDevices())
	}

	// Send DevicesToRemove.
	if err := stream.Send(devicepb.SyncInventoryRequest_builder{
		DevicesToRemove: devicepb.SyncInventoryDevices_builder{Devices: devsToRemove}.Build(),
	}.Build()); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("devicesToRemove Send: %w", err)
	}
	resp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("devicesToRemove Recv: %w", err)
	}
	statuses = append(statuses, resp.GetResult().GetDevices())

	// Signal end and wait for EOF.
	if err := stream.Send(devicepb.SyncInventoryRequest_builder{
		End: &devicepb.SyncInventoryEnd{},
	}.Build()); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("end Send: %w", err)
	}
	for {
		resp, err = stream.Recv()
		switch {
		case errors.Is(err, io.EOF):
			return statuses, nil
		case err != nil:
			return nil, fmt.Errorf("end Recv: %w", err)
		case resp.GetResult() == nil:
			return nil, fmt.Errorf("end: unexpected payload %T", resp.Payload)
		}

		// Unexpected, record and let the test figure it out.
		statuses = append(statuses, resp.GetResult().GetDevices())
	}
}

// syncInventoryMissing runs a missing devices-focused SyncInventory stream.
// It returns the collated statuses of the missing devices step.
func syncInventoryMissing(
	ctx context.Context,
	devicesClient devicepb.DeviceTrustServiceClient,
	source *devicepb.DeviceSource,
	osTypes []devicepb.OSType,
	devsToUpsert []*devicepb.Device,
	missingFn func([]*devicepb.Device) []*devicepb.Device,
) ([]*devicepb.DeviceOrStatus, error) {
	stream, err := devicesClient.SyncInventory(ctx)
	if err != nil {
		return nil, err
	}

	// Start.
	if err := stream.Send(devicepb.SyncInventoryRequest_builder{
		Start: devicepb.SyncInventoryStart_builder{
			Source:              source,
			TrackMissingDevices: true,
			OsTypes:             osTypes,
		}.Build(),
	}.Build()); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("start Send: %w", err)
	}

	// Ack.
	if _, err := stream.Recv(); err != nil {
		return nil, fmt.Errorf("start Recv: %w", err)
	}

	if len(devsToUpsert) > 0 {
		// Devices.
		if err := stream.Send(devicepb.SyncInventoryRequest_builder{
			DevicesToUpsert: devicepb.SyncInventoryDevices_builder{
				Devices: devsToUpsert,
			}.Build(),
		}.Build()); err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("devices Send: %w", err)
		}

		// Result.
		if _, err := stream.Recv(); err != nil {
			return nil, fmt.Errorf("devices Recv: %w", err)
		}
	}

	// End.
	if err := stream.Send(devicepb.SyncInventoryRequest_builder{
		End: &devicepb.SyncInventoryEnd{},
	}.Build()); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("end Send: %w", err)
	}

	// Missing devices.
	var statuses []*devicepb.DeviceOrStatus
	for {
		resp, err := stream.Recv() // MissingDevices
		if errors.Is(err, io.EOF) {
			break // Server closed the stream.
		}
		if err != nil {
			return nil, fmt.Errorf("missing Recv: %w", err)
		}

		// Echo deletions.
		if err := stream.Send(devicepb.SyncInventoryRequest_builder{
			DevicesToRemove: devicepb.SyncInventoryDevices_builder{
				Devices: missingFn(resp.GetMissingDevices().GetDevices()),
			}.Build(),
		}.Build()); err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("missing remove Send: %w", err)
		}

		// Deletion statuses.
		resp, err = stream.Recv()
		if err != nil {
			return nil, fmt.Errorf("missing result Recv: %w", err)
		}
		statuses = append(statuses, resp.GetResult().GetDevices()...)
	}
	return statuses, nil
}

func listAllDevices(ctx context.Context, devices devicepb.DeviceTrustServiceClient) ([]*devicepb.Device, error) {
	var devs []*devicepb.Device
	var pageToken string
	for {
		resp, err := devices.ListDevices(ctx, devicepb.ListDevicesRequest_builder{
			PageToken: pageToken,
			View:      devicepb.DeviceView_DEVICE_VIEW_RESOURCE,
		}.Build())
		if err != nil {
			return nil, err
		}

		devs = append(devs, resp.GetDevices()...)

		if resp.GetNextPageToken() == "" {
			return devs, nil
		}
		pageToken = resp.GetNextPageToken()
	}
}
