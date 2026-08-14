// Teleport
// Copyright (C) 2026  Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

use crate::scard::piv::tlv_tags;
use uuid::Uuid;

pub(super) fn build_chuid(uuid: Uuid) -> Vec<u8> {
    // This is gross: the response is a BER-TLV value, but it has nested SIMPLE-TLV
    // values. None of the TLV encoding libraries out there support this, they fail
    // when checking the tag of nested TLVs.
    //
    // So, construct the TLV by hand from raw bytes. Hopefully the comments will be
    // enough to explain the structure.
    //
    // https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf
    // table 9 has the explanation of fields.
    //
    // Start with a top-level BER-TLV tag and length:
    let mut resp = vec![tlv_tags::DATA_FIELD, 0x3B];
    // TLV tag and length for FASC-N.
    resp.extend_from_slice(&[tlv_tags::FASC_N, 0x19]);
    // FASC-N value containing S9999F9999F999999F0F1F0000000000300001E, with a
    // weird encoding from section 6 of:
    // https://www.idmanagement.gov/docs/pacs-tig-scepacs.pdf
    resp.extend_from_slice(&[
        0xd4, 0xe7, 0x39, 0xda, 0x73, 0x9c, 0xed, 0x39, 0xce, 0x73, 0x9d, 0x83, 0x68, 0x58, 0x21,
        0x08, 0x42, 0x10, 0x84, 0x21, 0xc8, 0x42, 0x10, 0xc3, 0xeb,
    ]);
    // TLV for user UUID.
    resp.extend_from_slice(&[tlv_tags::GUID, 0x10]);
    resp.extend_from_slice(uuid.as_bytes());
    // TLV for expiration date (YYYYMMDD).
    resp.extend_from_slice(&[tlv_tags::EXPIRATION_DATE, 0x08]);
    // TODO(awly): generate this from current time.
    resp.extend_from_slice("20300101".as_bytes());
    // TLV for signature (empty).
    resp.extend_from_slice(&[tlv_tags::ISSUER_ASYMMETRIC_SIGNATURE, 0x00]);
    // TLV for error detection code.
    resp.extend_from_slice(&[tlv_tags::ERROR_DETECTION_CODE, 0x00]);
    resp
}
