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

package types

import (
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
)

// CheckAndSetDefaults checks DeviceV1 fields to catch simple errors, and sets
// default values for all fields with defaults.
func (d *DeviceV1) CheckAndSetDefaults() error {
	if d == nil {
		return trace.BadParameter("device is nil")
	}

	// Assign defaults:
	// - Kind = device
	// - Metadata.Name = UUID
	// - Spec.EnrollStatus = unspecified
	// - Spec.Credential.AttestationType = unspecified
	if d.Kind == "" {
		d.Kind = KindDevice
	} else if d.Kind != KindDevice { // sanity check
		return trace.BadParameter("unexpected resource kind %q, must be %q", d.Kind, KindDevice)
	}
	if d.Metadata.Name == "" {
		d.Metadata.Name = uuid.NewString()
	}
	if d.Spec.EnrollStatus == "" {
		d.Spec.EnrollStatus = ResourceDeviceEnrollStatusToString(devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_UNSPECIFIED)
	}
	if d.Spec.Credential != nil && d.Spec.Credential.DeviceAttestationType == "" {
		d.Spec.Credential.DeviceAttestationType = ResourceDeviceAttestationTypeToString(devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_UNSPECIFIED)
	}

	// Validate Header/Metadata.
	if err := d.ResourceHeader.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	// Validate "simple" fields.
	switch {
	case d.Spec.OsType == "":
		return trace.BadParameter("missing OS type")
	case d.Spec.AssetTag == "":
		return trace.BadParameter("missing asset tag")
	}

	// Validate enum conversions.
	if _, err := ResourceOSTypeFromString(d.Spec.OsType); err != nil {
		return trace.Wrap(err)
	}
	if _, err := ResourceDeviceEnrollStatusFromString(d.Spec.EnrollStatus); err != nil {
		return trace.Wrap(err)
	}
	if d.Spec.Credential != nil {
		if _, err := ResourceDeviceAttestationTypeFromString(d.Spec.Credential.DeviceAttestationType); err != nil {
			return trace.Wrap(err)
		}
	}
	if d.Spec.Source != nil {
		if _, err := ResourceDeviceOriginFromString(d.Spec.Source.Origin); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// DeviceFromResource converts a resource DeviceV1 to an API devicepb.Device.
func DeviceFromResource(res *DeviceV1) (*devicepb.Device, error) {
	if res == nil {
		return nil, trace.BadParameter("device is nil")
	}

	toTimePB := func(t *time.Time) *timestamppb.Timestamp {
		if t == nil {
			return nil
		}
		return timestamppb.New(*t)
	}

	osType, err := ResourceOSTypeFromString(res.Spec.OsType)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	enrollStatus, err := ResourceDeviceEnrollStatusFromString(res.Spec.EnrollStatus)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var cred *devicepb.DeviceCredential
	if res.Spec.Credential != nil {
		attestationType, err := ResourceDeviceAttestationTypeFromString(
			res.Spec.Credential.DeviceAttestationType,
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cred = &devicepb.DeviceCredential{
			Id:                    res.Spec.Credential.Id,
			PublicKeyDer:          res.Spec.Credential.PublicKeyDer,
			DeviceAttestationType: attestationType,
			TpmEkcertSerial:       res.Spec.Credential.TpmEkcertSerial,
			TpmAkPublic:           res.Spec.Credential.TpmAkPublic,
		}
	}

	collectedData := make([]*devicepb.DeviceCollectedData, len(res.Spec.CollectedData))
	for i, d := range res.Spec.CollectedData {
		dataOSType, err := ResourceOSTypeFromString(d.OsType)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		collectedData[i] = &devicepb.DeviceCollectedData{
			CollectTime:             toTimePB(d.CollectTime),
			RecordTime:              toTimePB(d.RecordTime),
			OsType:                  dataOSType,
			SerialNumber:            d.SerialNumber,
			ModelIdentifier:         d.ModelIdentifier,
			OsVersion:               d.OsVersion,
			OsBuild:                 d.OsBuild,
			OsUsername:              d.OsUsername,
			JamfBinaryVersion:       d.JamfBinaryVersion,
			MacosEnrollmentProfiles: d.MacosEnrollmentProfiles,
			ReportedAssetTag:        d.ReportedAssetTag,
			SystemSerialNumber:      d.SystemSerialNumber,
			BaseBoardSerialNumber:   d.BaseBoardSerialNumber,
			TpmPlatformAttestation: tpmPlatformAttestationFromResource(
				d.TpmPlatformAttestation,
			),
			OsId: d.OsId,
		}
	}

	var source *devicepb.DeviceSource
	if s := res.Spec.Source; s != nil {
		origin, err := ResourceDeviceOriginFromString(s.Origin)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		source = &devicepb.DeviceSource{
			Name:   s.Name,
			Origin: origin,
		}
	}

	var profile *devicepb.DeviceProfile
	if p := res.Spec.Profile; p != nil {
		profile = &devicepb.DeviceProfile{
			UpdateTime:          toTimePB(p.UpdateTime),
			ModelIdentifier:     p.ModelIdentifier,
			OsVersion:           p.OsVersion,
			OsBuild:             p.OsBuild,
			OsBuildSupplemental: p.OsBuildSupplemental,
			OsUsernames:         p.OsUsernames,
			JamfBinaryVersion:   p.JamfBinaryVersion,
			ExternalId:          p.ExternalId,
			OsId:                p.OsId,
		}
	}

	return &devicepb.Device{
		ApiVersion:    res.Version,
		Id:            res.Metadata.Name,
		OsType:        osType,
		AssetTag:      res.Spec.AssetTag,
		CreateTime:    toTimePB(res.Spec.CreateTime),
		UpdateTime:    toTimePB(res.Spec.UpdateTime),
		EnrollStatus:  enrollStatus,
		Credential:    cred,
		CollectedData: collectedData,
		Source:        source,
		Profile:       profile,
		Owner:         res.Spec.Owner,
	}, nil
}

// DeviceToResource converts an API devicepb.Device to a resource DeviceV1 and
// assigns all default fields.
func DeviceToResource(dev *devicepb.Device) *DeviceV1 {
	if dev == nil {
		return nil
	}

	toTimePtr := func(pb *timestamppb.Timestamp) *time.Time {
		if pb == nil {
			return nil
		}
		t := pb.AsTime()
		return &t
	}

	var cred *DeviceCredential
	if dev.Credential != nil {
		cred = &DeviceCredential{
			Id:           dev.Credential.Id,
			PublicKeyDer: dev.Credential.PublicKeyDer,
			DeviceAttestationType: ResourceDeviceAttestationTypeToString(
				dev.Credential.DeviceAttestationType,
			),
			TpmEkcertSerial: dev.Credential.TpmEkcertSerial,
			TpmAkPublic:     dev.Credential.TpmAkPublic,
		}
	}

	collectedData := make([]*DeviceCollectedData, len(dev.CollectedData))
	for i, d := range dev.CollectedData {
		collectedData[i] = &DeviceCollectedData{
			CollectTime:             toTimePtr(d.CollectTime),
			RecordTime:              toTimePtr(d.RecordTime),
			OsType:                  ResourceOSTypeToString(d.OsType),
			SerialNumber:            d.SerialNumber,
			ModelIdentifier:         d.ModelIdentifier,
			OsVersion:               d.OsVersion,
			OsBuild:                 d.OsBuild,
			OsUsername:              d.OsUsername,
			JamfBinaryVersion:       d.JamfBinaryVersion,
			MacosEnrollmentProfiles: d.MacosEnrollmentProfiles,
			ReportedAssetTag:        d.ReportedAssetTag,
			SystemSerialNumber:      d.SystemSerialNumber,
			BaseBoardSerialNumber:   d.BaseBoardSerialNumber,
			TpmPlatformAttestation: tpmPlatformAttestationToResource(
				d.TpmPlatformAttestation,
			),
			OsId: d.OsId,
		}
	}

	var source *DeviceSource
	if s := dev.Source; s != nil {
		source = &DeviceSource{
			Name:   s.Name,
			Origin: ResourceDeviceOriginToString(s.Origin),
		}
	}

	var profile *DeviceProfile
	if p := dev.Profile; p != nil {
		profile = &DeviceProfile{
			UpdateTime:          toTimePtr(p.UpdateTime),
			ModelIdentifier:     p.ModelIdentifier,
			OsVersion:           p.OsVersion,
			OsBuild:             p.OsBuild,
			OsBuildSupplemental: p.OsBuildSupplemental,
			OsUsernames:         p.OsUsernames,
			JamfBinaryVersion:   p.JamfBinaryVersion,
			ExternalId:          p.ExternalId,
			OsId:                p.OsId,
		}
	}

	res := &DeviceV1{
		ResourceHeader: ResourceHeader{
			Kind:    KindDevice,
			Version: dev.ApiVersion,
			Metadata: Metadata{
				Name: dev.Id,
			},
		},
		Spec: &DeviceSpec{
			OsType:        ResourceOSTypeToString(dev.OsType),
			AssetTag:      dev.AssetTag,
			CreateTime:    toTimePtr(dev.CreateTime),
			UpdateTime:    toTimePtr(dev.UpdateTime),
			EnrollStatus:  ResourceDeviceEnrollStatusToString(dev.EnrollStatus),
			Credential:    cred,
			CollectedData: collectedData,
			Source:        source,
			Profile:       profile,
			Owner:         dev.Owner,
		},
	}
	_ = res.CheckAndSetDefaults() // assign default fields
	return res
}

func tpmPlatformAttestationToResource(pa *devicepb.TPMPlatformAttestation) *TPMPlatformAttestation {
	if pa == nil {
		return nil
	}

	var outPlatParams *TPMPlatformParameters
	if pa.PlatformParameters != nil {
		var quotes []*TPMQuote
		for _, q := range pa.PlatformParameters.Quotes {
			quotes = append(quotes, &TPMQuote{
				Quote:     q.Quote,
				Signature: q.Signature,
			})
		}

		var pcrs []*TPMPCR
		for _, pcr := range pa.PlatformParameters.Pcrs {
			pcrs = append(pcrs, &TPMPCR{
				Index:     pcr.Index,
				Digest:    pcr.Digest,
				DigestAlg: pcr.DigestAlg,
			})
		}

		outPlatParams = &TPMPlatformParameters{
			Quotes:   quotes,
			Pcrs:     pcrs,
			EventLog: pa.PlatformParameters.EventLog,
		}
	}

	return &TPMPlatformAttestation{
		Nonce:              pa.Nonce,
		PlatformParameters: outPlatParams,
	}
}

func tpmPlatformAttestationFromResource(pa *TPMPlatformAttestation) *devicepb.TPMPlatformAttestation {
	if pa == nil {
		return nil
	}

	var outPlatParams *devicepb.TPMPlatformParameters
	if pa.PlatformParameters != nil {
		var quotes []*devicepb.TPMQuote
		for _, q := range pa.PlatformParameters.Quotes {
			quotes = append(quotes, &devicepb.TPMQuote{
				Quote:     q.Quote,
				Signature: q.Signature,
			})
		}

		var pcrs []*devicepb.TPMPCR
		for _, pcr := range pa.PlatformParameters.Pcrs {
			pcrs = append(pcrs, &devicepb.TPMPCR{
				Index:     pcr.Index,
				Digest:    pcr.Digest,
				DigestAlg: pcr.DigestAlg,
			})
		}

		outPlatParams = &devicepb.TPMPlatformParameters{
			Quotes:   quotes,
			EventLog: pa.PlatformParameters.EventLog,
			Pcrs:     pcrs,
		}
	}

	return &devicepb.TPMPlatformAttestation{
		Nonce:              pa.Nonce,
		PlatformParameters: outPlatParams,
	}
}

// ResourceOSTypeToString converts OSType to a string representation suitable
// for use in resource fields.
func ResourceOSTypeToString(osType devicepb.OSType) string {
	switch osType {
	case devicepb.OSType_OS_TYPE_UNSPECIFIED:
		return "unspecified"
	case devicepb.OSType_OS_TYPE_LINUX:
		return "linux"
	case devicepb.OSType_OS_TYPE_MACOS:
		return "macos"
	case devicepb.OSType_OS_TYPE_WINDOWS:
		return "windows"
	default:
		return osType.String()
	}
}

// ResourceOSTypeFromString converts a string representation of OSType suitable
// for resource fields to OSType.
func ResourceOSTypeFromString(osType string) (devicepb.OSType, error) {
	switch osType {
	case "", "unspecified":
		return devicepb.OSType_OS_TYPE_UNSPECIFIED, nil
	case "linux":
		return devicepb.OSType_OS_TYPE_LINUX, nil
	case "macos":
		return devicepb.OSType_OS_TYPE_MACOS, nil
	case "windows":
		return devicepb.OSType_OS_TYPE_WINDOWS, nil
	default:
		return devicepb.OSType_OS_TYPE_UNSPECIFIED, trace.BadParameter("unknown os type %q", osType)
	}
}

// ResourceDeviceEnrollStatusToString converts DeviceEnrollStatus to a string
// representation suitable for use in resource fields.
func ResourceDeviceEnrollStatusToString(enrollStatus devicepb.DeviceEnrollStatus) string {
	switch enrollStatus {
	case devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED:
		return "enrolled"
	case devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED:
		return "not_enrolled"
	case devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_UNSPECIFIED:
		return "unspecified"
	default:
		return enrollStatus.String()
	}
}

// ResourceDeviceEnrollStatusFromString converts a string representation of
// DeviceEnrollStatus suitable for resource fields to DeviceEnrollStatus.
func ResourceDeviceEnrollStatusFromString(enrollStatus string) (devicepb.DeviceEnrollStatus, error) {
	switch enrollStatus {
	case "enrolled":
		return devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_ENROLLED, nil
	case "not_enrolled":
		return devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_NOT_ENROLLED, nil
	case "unspecified":
		return devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_UNSPECIFIED, nil
	// In the terraform provider, enroll_status is an optional field and can be empty.
	case "":
		return devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_UNSPECIFIED, nil
	default:
		return devicepb.DeviceEnrollStatus_DEVICE_ENROLL_STATUS_UNSPECIFIED, trace.BadParameter("unknown enroll status %q", enrollStatus)
	}
}

func ResourceDeviceAttestationTypeToString(
	attestationType devicepb.DeviceAttestationType,
) string {
	switch attestationType {
	case devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_UNSPECIFIED:
		// Default to empty, so it doesn't show in non-TPM devices.
		return ""
	case devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_TPM_EKPUB:
		return "tpm_ekpub"
	case devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_TPM_EKCERT:
		return "tpm_ekcert"
	case devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_TPM_EKCERT_TRUSTED:
		return "tpm_ekcert_trusted"
	default:
		return attestationType.String()
	}
}

func ResourceDeviceAttestationTypeFromString(
	attestationType string,
) (devicepb.DeviceAttestationType, error) {
	switch attestationType {
	case "unspecified", "":
		return devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_UNSPECIFIED, nil
	case "tpm_ekpub":
		return devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_TPM_EKPUB, nil
	case "tpm_ekcert":
		return devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_TPM_EKCERT, nil
	case "tpm_ekcert_trusted":
		return devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_TPM_EKCERT_TRUSTED, nil
	default:
		return devicepb.DeviceAttestationType_DEVICE_ATTESTATION_TYPE_UNSPECIFIED, trace.BadParameter("unknown attestation type %q", attestationType)
	}
}

func ResourceDeviceOriginToString(o devicepb.DeviceOrigin) string {
	switch o {
	case devicepb.DeviceOrigin_DEVICE_ORIGIN_UNSPECIFIED:
		return "unspecified"
	case devicepb.DeviceOrigin_DEVICE_ORIGIN_API:
		return "api"
	case devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF:
		return "jamf"
	case devicepb.DeviceOrigin_DEVICE_ORIGIN_INTUNE:
		return "intune"
	default:
		return o.String()
	}
}

func ResourceDeviceOriginFromString(s string) (devicepb.DeviceOrigin, error) {
	switch s {
	case "", "unspecified":
		return devicepb.DeviceOrigin_DEVICE_ORIGIN_UNSPECIFIED, nil
	case "api":
		return devicepb.DeviceOrigin_DEVICE_ORIGIN_API, nil
	case "jamf":
		return devicepb.DeviceOrigin_DEVICE_ORIGIN_JAMF, nil
	case "intune":
		return devicepb.DeviceOrigin_DEVICE_ORIGIN_INTUNE, nil
	default:
		return devicepb.DeviceOrigin_DEVICE_ORIGIN_UNSPECIFIED, trace.BadParameter("unknown device origin %q", s)
	}
}
