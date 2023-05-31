package jamf_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/gravitational/teleport/e/lib/jamf"
)

const (
	// getComputersInventoryResponseAPIExample is the example from
	// https://developer.jamf.com/jamf-pro/reference/get_v1-computers-inventory.
	getComputersInventoryResponseAPIExample = `{
  "totalCount": 3,
  "results": [
    {
      "id": "1",
      "udid": "123",
      "general": {
        "name": "Boalime",
        "lastIpAddress": "247.185.82.186",
        "lastReportedIp": "247.185.82.186",
        "jamfBinaryVersion": "9.27",
        "platform": "Mac",
        "barcode1": "5 12345 678900",
        "barcode2": "5 12345 678900",
        "assetTag": "304822",
        "remoteManagement": {
          "managed": true,
          "managementUsername": "rootname"
        },
        "supervised": true,
        "mdmCapable": {
          "capable": true,
          "capableUsers": [
            "admin",
            "rootadmin"
          ]
        },
        "reportDate": "2018-10-31T18:04:13Z",
        "lastContactTime": "2018-10-31T18:04:13Z",
        "lastCloudBackupDate": "2018-10-31T18:04:13Z",
        "lastEnrolledDate": "2018-10-31T18:04:13Z",
        "mdmProfileExpiration": "2018-10-31T18:04:13Z",
        "initialEntryDate": "2018-10-31",
        "distributionPoint": "distribution point name",
        "enrollmentMethod": {
          "id": "1",
          "objectName": "user@domain.com",
          "objectType": "User-initiated - no invitation"
        },
        "site": {
          "id": "1",
          "name": "Eau Claire"
        },
        "itunesStoreAccountActive": true,
        "enrolledViaAutomatedDeviceEnrollment": true,
        "userApprovedMdm": true,
        "declarativeDeviceManagementEnabled": true,
        "extensionAttributes": [
          {
            "definitionId": "23",
            "name": "Some Attribute",
            "description": "Some Attribute defines how much Foo impacts Bar.",
            "enabled": true,
            "multiValue": true,
            "values": [
              "foo",
              "bar"
            ],
            "dataType": "STRING",
            "options": [
              "foo",
              "bar"
            ],
            "inputType": "TEXT"
          }
        ],
        "managementId": "73226fb6-61df-4c10-9552-eb9bc353d507"
      },
      "diskEncryption": {
        "bootPartitionEncryptionDetails": {
          "partitionName": "main",
          "partitionFileVault2State": "VALID",
          "partitionFileVault2Percent": 100
        },
        "individualRecoveryKeyValidityStatus": "VALID",
        "institutionalRecoveryKeyPresent": true,
        "diskEncryptionConfigurationName": "Test configuration",
        "fileVault2EnabledUserNames": [
          "admin"
        ],
        "fileVault2EligibilityMessage": "Not a boot partition"
      },
      "purchasing": {
        "leased": true,
        "purchased": true,
        "poNumber": "53-1",
        "poDate": "2019-01-01",
        "vendor": "Example Vendor",
        "warrantyDate": "2019-01-01",
        "appleCareId": "abcd",
        "leaseDate": "2019-01-01",
        "purchasePrice": "$500",
        "lifeExpectancy": 5,
        "purchasingAccount": "admin",
        "purchasingContact": "true",
        "extensionAttributes": [
          {
            "definitionId": "23",
            "name": "Some Attribute",
            "description": "Some Attribute defines how much Foo impacts Bar.",
            "enabled": true,
            "multiValue": true,
            "values": [
              "foo",
              "bar"
            ],
            "dataType": "STRING",
            "options": [
              "foo",
              "bar"
            ],
            "inputType": "TEXT"
          }
        ]
      },
      "applications": [
        {
          "name": "Microsoft Word",
          "path": "/usr/local/app",
          "version": "1.0.0",
          "macAppStore": true,
          "sizeMegabytes": 25,
          "bundleId": "1",
          "updateAvailable": false,
          "externalVersionId": "1"
        }
      ],
      "storage": {
        "bootDriveAvailableSpaceMegabytes": 3072,
        "disks": [
          {
            "id": "170",
            "device": "disk0",
            "model": "APPLE HDD TOSHIBA MK5065GSXF",
            "revision": "5",
            "serialNumber": "a8598f013366",
            "sizeMegabytes": 262144,
            "smartStatus": "OK",
            "type": "false",
            "partitions": [
              {
                "name": "Foo",
                "sizeMegabytes": 262144,
                "availableMegabytes": 131072,
                "partitionType": "BOOT",
                "percentUsed": 25,
                "fileVault2State": "VALID",
                "fileVault2ProgressPercent": 45,
                "lvmManaged": true
              }
            ]
          }
        ]
      },
      "userAndLocation": {
        "username": "Madison Anderson",
        "realname": "13-inch MacBook",
        "email": "email@com.pl",
        "position": "IT Team Lead",
        "phone": "123-456-789",
        "departmentId": "1",
        "buildingId": "1",
        "room": "5",
        "extensionAttributes": [
          {
            "definitionId": "23",
            "name": "Some Attribute",
            "description": "Some Attribute defines how much Foo impacts Bar.",
            "enabled": true,
            "multiValue": true,
            "values": [
              "foo",
              "bar"
            ],
            "dataType": "STRING",
            "options": [
              "foo",
              "bar"
            ],
            "inputType": "TEXT"
          }
        ]
      },
      "configurationProfiles": [
        {
          "id": "1",
          "username": "username",
          "lastInstalled": "2018-10-31T18:04:13Z",
          "removable": true,
          "displayName": "Displayed profile",
          "profileIdentifier": "0ae590fe-9b30-11ea-bb37-0242ac130002"
        }
      ],
      "printers": [
        {
          "name": "My Printer",
          "type": "XYZ 1122",
          "uri": "ipp://10.0.0.5",
          "location": "7th floor"
        }
      ],
      "services": [
        {
          "name": "SomeService"
        }
      ],
      "hardware": {
        "make": "Apple",
        "model": "13-inch MacBook Pro (Mid 2012)",
        "modelIdentifier": "MacBookPro9,2",
        "serialNumber": "C02ZC2QYLVDL",
        "processorSpeedMhz": 2100,
        "processorCount": 2,
        "coreCount": 2,
        "processorType": "Intel Core i5",
        "processorArchitecture": "i386",
        "busSpeedMhz": 2133,
        "cacheSizeKilobytes": 3072,
        "networkAdapterType": "Foo",
        "macAddress": "6A:2C:4B:B7:65:B5",
        "altNetworkAdapterType": "Bar",
        "altMacAddress": "82:45:58:44:dc:01",
        "totalRamMegabytes": 4096,
        "openRamSlots": 0,
        "batteryCapacityPercent": 85,
        "smcVersion": "2.2f38",
        "nicSpeed": "N/A",
        "opticalDrive": "MATSHITA DVD-R UJ-8A8",
        "bootRom": "MBP91.00D3.B08",
        "bleCapable": false,
        "supportsIosAppInstalls": false,
        "appleSilicon": false,
        "extensionAttributes": [
          {
            "definitionId": "23",
            "name": "Some Attribute",
            "description": "Some Attribute defines how much Foo impacts Bar.",
            "enabled": true,
            "multiValue": true,
            "values": [
              "foo",
              "bar"
            ],
            "dataType": "STRING",
            "options": [
              "foo",
              "bar"
            ],
            "inputType": "TEXT"
          }
        ]
      },
      "localUserAccounts": [
        {
          "uid": "501",
          "userGuid": "844F1177-0CF5-40C6-901F-38EDD9969C1C",
          "username": "jamf",
          "fullName": "John Jamf",
          "admin": true,
          "homeDirectory": "/Users/jamf",
          "homeDirectorySizeMb": 131072,
          "fileVault2Enabled": true,
          "userAccountType": "LOCAL",
          "passwordMinLength": 4,
          "passwordMaxAge": 5,
          "passwordMinComplexCharacters": 5,
          "passwordHistoryDepth": 5,
          "passwordRequireAlphanumeric": true,
          "computerAzureActiveDirectoryId": "1",
          "userAzureActiveDirectoryId": "1",
          "azureActiveDirectoryId": "ACTIVATED"
        }
      ],
      "certificates": [
        {
          "commonName": "jamf.com",
          "identity": true,
          "expirationDate": "2030-10-31T18:04:13Z",
          "username": "test",
          "lifecycleStatus": "ACTIVE",
          "certificateStatus": "ISSUED",
          "subjectName": "CN=jamf.com",
          "serialNumber": "40f3d9fb",
          "sha1Fingerprint": "ed361458724d06082b2314acdb82e1f586f085f5",
          "issuedDate": "2022-05-23T14:54:10Z"
        }
      ],
      "attachments": [
        {
          "id": "1",
          "name": "Attachment.pdf",
          "fileType": "application/pdf",
          "sizeBytes": 1024
        }
      ],
      "plugins": [
        {
          "name": "plugin name",
          "version": "1.02",
          "path": "/Applications/"
        }
      ],
      "packageReceipts": {
        "installedByJamfPro": [
          "com.jamf.protect.JamfProtect"
        ],
        "installedByInstallerSwu": [
          "com.apple.pkg.Core"
        ],
        "cached": [
          "com.jamf.protect.JamfProtect"
        ]
      },
      "fonts": [
        {
          "name": "font name",
          "version": "1.02",
          "path": "/Applications/"
        }
      ],
      "security": {
        "sipStatus": "ENABLED",
        "gatekeeperStatus": "APP_STORE_AND_IDENTIFIED_DEVELOPERS",
        "xprotectVersion": "1.2.3",
        "autoLoginDisabled": false,
        "remoteDesktopEnabled": true,
        "activationLockEnabled": true,
        "recoveryLockEnabled": true,
        "firewallEnabled": true,
        "secureBootLevel": "FULL_SECURITY",
        "externalBootLevel": "ALLOW_BOOTING_FROM_EXTERNAL_MEDIA",
        "bootstrapTokenAllowed": true
      },
      "operatingSystem": {
        "name": "Mac OS X",
        "version": "10.9.5",
        "build": "13A603",
        "supplementalBuildVersion": "13A953",
        "rapidSecurityResponse": "(a)",
        "activeDirectoryStatus": "Not Bound",
        "fileVault2Status": "ALL_ENCRYPTED",
        "softwareUpdateDeviceId": "J132AP",
        "extensionAttributes": [
          {
            "definitionId": "23",
            "name": "Some Attribute",
            "description": "Some Attribute defines how much Foo impacts Bar.",
            "enabled": true,
            "multiValue": true,
            "values": [
              "foo",
              "bar"
            ],
            "dataType": "STRING",
            "options": [
              "foo",
              "bar"
            ],
            "inputType": "TEXT"
          }
        ]
      },
      "licensedSoftware": [
        {
          "id": "1",
          "name": "Microsoft Word"
        }
      ],
      "ibeacons": [
        {
          "name": "room A"
        }
      ],
      "softwareUpdates": [
        {
          "name": "BEdit",
          "version": "1.15.2",
          "packageName": "com.apple.pkg.AdditionalEssentials"
        }
      ],
      "extensionAttributes": [
        {
          "definitionId": "23",
          "name": "Some Attribute",
          "description": "Some Attribute defines how much Foo impacts Bar.",
          "enabled": true,
          "multiValue": true,
          "values": [
            "foo",
            "bar"
          ],
          "dataType": "STRING",
          "options": [
            "foo",
            "bar"
          ],
          "inputType": "TEXT"
        }
      ],
      "contentCaching": {
        "computerContentCachingInformationId": "1",
        "parents": [
          {
            "contentCachingParentId": "1",
            "address": "SomeAddress",
            "alerts": {
              "contentCachingParentAlertId": "1",
              "addresses": [],
              "className": "SomeClass",
              "postDate": "2018-10-31T18:04:13Z"
            },
            "details": {
              "contentCachingParentDetailsId": "1",
              "acPower": true,
              "cacheSizeBytes": 0,
              "capabilities": {
                "contentCachingParentCapabilitiesId": "1",
                "imports": true,
                "namespaces": true,
                "personalContent": true,
                "queryParameters": true,
                "sharedContent": true,
                "prioritization": true
              },
              "portable": true,
              "localNetwork": [
                {
                  "contentCachingParentLocalNetworkId": "1",
                  "speed": 5000,
                  "wired": true
                }
              ]
            },
            "guid": "CD1E1291-4AF9-4468-B5D5-0F780C13DB2F",
            "healthy": true,
            "port": 0,
            "version": "1"
          }
        ],
        "alerts": [
          {
            "cacheBytesLimit": 0,
            "className": "SomeClass",
            "pathPreventingAccess": "/some/path",
            "postDate": "2018-10-31T18:04:13Z",
            "reservedVolumeBytes": 0,
            "resource": "SomeResource"
          }
        ],
        "activated": false,
        "active": false,
        "actualCacheBytesUsed": 0,
        "cacheDetails": [
          {
            "computerContentCachingCacheDetailsId": "1",
            "categoryName": "SomeCategory",
            "diskSpaceBytesUsed": 0
          }
        ],
        "cacheBytesFree": 23353884672,
        "cacheBytesLimit": 0,
        "cacheStatus": "OK",
        "cacheBytesUsed": 0,
        "dataMigrationCompleted": false,
        "dataMigrationProgressPercentage": 0,
        "dataMigrationError": {
          "code": 0,
          "domain": "SomeDomain",
          "userInfo": [
            {
              "key": "foo",
              "value": "bar"
            }
          ]
        },
        "maxCachePressureLast1HourPercentage": 0,
        "personalCacheBytesFree": 23353884672,
        "personalCacheBytesLimit": 0,
        "personalCacheBytesUsed": 0,
        "port": 0,
        "publicAddress": "SomeAddress",
        "registrationError": "NOT_ACTIVATED",
        "registrationResponseCode": 403,
        "registrationStarted": "2018-10-31T18:04:13Z",
        "registrationStatus": "CONTENT_CACHING_FAILED",
        "restrictedMedia": false,
        "serverGuid": "CD1E1291-4AF9-4468-B5D5-0F780C13DB2F",
        "startupStatus": "FAILED",
        "tetheratorStatus": "CONTENT_CACHING_DISABLED",
        "totalBytesAreSince": "2018-10-31T18:04:13Z",
        "totalBytesDropped": 0,
        "totalBytesImported": 0,
        "totalBytesReturnedToChildren": 0,
        "totalBytesReturnedToClients": 0,
        "totalBytesReturnedToPeers": 0,
        "totalBytesStoredFromOrigin": 0,
        "totalBytesStoredFromParents": 0,
        "totalBytesStoredFromPeers": 0
      },
      "groupMemberships": [
        {
          "groupId": "1",
          "groupName": "groupOne",
          "smartGroup": true
        }
      ]
    }
  ]
}`

	// getComputersInventoryResponseRealExample is a redacted example from a
	// test account.
	getComputersInventoryResponseRealExample = `{
  "totalCount" : 1,
  "results" : [ {
    "id" : "9",
    "udid" : "54C764F5-7DC6-5CC0-B877-641BEDF8AF20",
    "general" : {
      "name" : "llama’s MacBook Air",
      "lastIpAddress" : "172.217.162.100",
      "lastReportedIp" : "192.168.1.38",
      "jamfBinaryVersion" : "10.43.1-t1674743888",
      "platform" : "Mac",
      "barcode1" : null,
      "barcode2" : null,
      "assetTag" : null,
      "remoteManagement" : {
        "managed" : true,
        "managementUsername" : "teleport"
      },
      "supervised" : true,
      "mdmCapable" : {
        "capable" : false,
        "capableUsers" : [ ]
      },
      "reportDate" : "2023-02-15T14:07:45.348Z",
      "lastContactTime" : "2023-02-21T09:31:49.792Z",
      "lastCloudBackupDate" : null,
      "lastEnrolledDate" : "2023-02-15T13:47:10.368Z",
      "mdmProfileExpiration" : "2025-02-15T13:46:35Z",
      "initialEntryDate" : "2023-02-15",
      "distributionPoint" : null,
      "site" : {
        "id" : "-1",
        "name" : "None"
      },
      "itunesStoreAccountActive" : true,
      "enrolledViaAutomatedDeviceEnrollment" : false,
      "userApprovedMdm" : false,
      "enrollmentMethod" : {
        "id" : "9",
        "objectName" : null,
        "objectType" : "User-initiated - no invitation"
      },
      "declarativeDeviceManagementEnabled" : false,
      "managementId" : "cbc948d5-16ab-4ca6-97bf-f78005c3311c",
      "extensionAttributes" : [ {
        "definitionId" : "6",
        "name" : "teleport_enrolment_token",
        "description" : null,
        "values" : [ "54954044-b8f7-42b7-8b39-5649b7989847" ],
        "dataType" : "STRING",
        "options" : [ ],
        "inputType" : "TEXT",
        "enabled" : true,
        "multiValue" : false
      } ]
    },
    "diskEncryption" : null,
    "localUserAccounts" : [ {
      "uid" : "501",
      "username" : "llama",
      "fullName" : "llama",
      "admin" : true,
      "userAccountType" : "UNKNOWN",
      "homeDirectory" : "/Users/llama",
      "homeDirectorySizeMb" : -1,
      "fileVault2Enabled" : true,
      "passwordMinLength" : 4,
      "passwordMaxAge" : null,
      "passwordMinComplexCharacters" : null,
      "passwordRequireAlphanumeric" : false,
      "passwordHistoryDepth" : null,
      "computerAzureActiveDirectoryId" : null,
      "userAzureActiveDirectoryId" : null,
      "azureActiveDirectoryId" : null
    } ],
    "purchasing" : null,
    "printers" : null,
    "storage" : null,
    "applications" : null,
    "userAndLocation" : null,
    "configurationProfiles" : null,
    "services" : null,
    "plugins" : null,
    "hardware" : {
      "make" : "Apple",
      "model" : "MacBook Air (M1, 2020)",
      "modelIdentifier" : "MacBookAir10,1",
      "serialNumber" : "XXXXXXXXXXXX",
      "processorSpeedMhz" : 0,
      "processorCount" : 1,
      "coreCount" : 8,
      "processorType" : "Apple M1",
      "processorArchitecture" : "arm64",
      "busSpeedMhz" : 0,
      "cacheSizeKilobytes" : 0,
      "networkAdapterType" : "IEEE80211",
      "macAddress" : "00:00:00:00:00:00",
      "altNetworkAdapterType" : "Ethernet",
      "altMacAddress" : "00:00:00:00:00:00",
      "totalRamMegabytes" : 8192,
      "openRamSlots" : 0,
      "batteryCapacityPercent" : 2,
      "smcVersion" : null,
      "nicSpeed" : "n/a",
      "opticalDrive" : null,
      "bootRom" : "8419.60.44",
      "bleCapable" : false,
      "supportsIosAppInstalls" : true,
      "appleSilicon" : true,
      "extensionAttributes" : [ ]
    },
    "certificates" : null,
    "attachments" : null,
    "packageReceipts" : null,
    "fonts" : null,
    "security" : null,
    "operatingSystem" : null,
    "licensedSoftware" : null,
    "softwareUpdates" : null,
    "groupMemberships" : null,
    "extensionAttributes" : null,
    "contentCaching" : null,
    "ibeacons" : null
  } ]
}`
)

func TestJSONUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		example string
		message any
		want    any
	}{
		{
			// Example based on manually forced API errors.
			name: "APIError",
			example: `{
  "httpStatus" : 401,
  "errors" : [ {
    "code" : "INVALID_TOKEN",
    "description" : "Unauthorized",
    "id" : "0",
    "field" : null
  } ]
}`,
			message: &jamf.APIError{},
			want:    &jamf.APIError{}, // nothing mapped atm
		},
		{
			// Example based on
			// https://developer.jamf.com/jamf-pro/reference/post_v1-auth-token.
			name: "AuthToken",
			example: `{
  "token": "eyJhbGciOiJIUzUxMiJ9...",
  "expires": "2020-04-21T21:09:31.626Z"
}`,
			message: &jamf.AuthToken{},
			want: &jamf.AuthToken{
				Token:   `eyJhbGciOiJIUzUxMiJ9...`,
				Expires: time.Date(2020, 4, 21, 21, 9, 31, 626000000, time.UTC),
			},
		},
		{
			name:    "GetComputersInventoryResponse API example",
			example: getComputersInventoryResponseAPIExample,
			message: &jamf.GetComputersInventoryResponse{},
			want: &jamf.GetComputersInventoryResponse{
				TotalCount: 3,
				Results: []*jamf.ComputerInventory{
					{
						ID:   "1",
						UDID: "123",
						General: &jamf.ComputerGeneralSection{
							JamfBinaryVersion: "9.27",
							Platform:          "Mac",
							ReportDate:        time.Date(2018, 10, 31, 18, 4, 13, 0, time.UTC),
							LastContactTime:   time.Date(2018, 10, 31, 18, 4, 13, 0, time.UTC),
							LastEnrolledDate:  time.Date(2018, 10, 31, 18, 4, 13, 0, time.UTC),
						},
						Hardware: &jamf.ComputerHardwareSection{
							ModelIdentifier: "MacBookPro9,2",
							SerialNumber:    "C02ZC2QYLVDL",
						},
						LocalUserAccounts: []*jamf.LocalUserAccount{
							{
								UID:      "501",
								Username: "jamf",
								FullName: "John Jamf",
							},
						},
						OperatingSystem: &jamf.ComputerOperatingSystemSection{
							Name:    "Mac OS X",
							Version: "10.9.5",
							Build:   "13A603",
						},
					},
				},
			},
		},
		{
			name:    "GetComputersInventoryResponse real example",
			example: getComputersInventoryResponseRealExample,
			message: &jamf.GetComputersInventoryResponse{},
			want: &jamf.GetComputersInventoryResponse{
				TotalCount: 1,
				Results: []*jamf.ComputerInventory{
					{
						ID:   "9",
						UDID: "54C764F5-7DC6-5CC0-B877-641BEDF8AF20",
						General: &jamf.ComputerGeneralSection{
							JamfBinaryVersion: "10.43.1-t1674743888",
							Platform:          "Mac",
							ReportDate:        time.Date(2023, 2, 15, 14, 7, 45, 348000000, time.UTC),
							LastContactTime:   time.Date(2023, 2, 21, 9, 31, 49, 792000000, time.UTC),
							LastEnrolledDate:  time.Date(2023, 2, 15, 13, 47, 10, 368000000, time.UTC),
						},
						Hardware: &jamf.ComputerHardwareSection{
							ModelIdentifier: "MacBookAir10,1",
							SerialNumber:    "XXXXXXXXXXXX",
						},
						LocalUserAccounts: []*jamf.LocalUserAccount{
							{
								UID:      "501",
								Username: "llama",
								FullName: "llama",
							},
						},
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(test.example), test.message); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if diff := cmp.Diff(test.want, test.message); diff != "" {
				t.Errorf("Unmarshal mismatch (-want +got)\n%s", diff)
			}
		})
	}
}
