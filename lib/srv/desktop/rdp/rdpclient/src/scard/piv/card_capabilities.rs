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

use rand::{RngCore, TryRngCore};

use crate::scard::piv::tlv_tags;

pub(super) fn build_ccc() -> Vec<u8> {
    // Card Capability Container, described in subsection 3.1.1. here: https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf#page=18
    // Its structure is defined in Table 8 here: https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-73-4.pdf#page=40
    // The description of each field can be found here: https://nvlpubs.nist.gov/nistpubs/Legacy/IR/nistir6887e2003.pdf#page=117
    // The CCC is comprised of SIMPLE-TLV data objects.
    let mut ccc = Vec::new();

    const TLV_L_IDX: usize = 1;
    const TLV_TL_SIZE: usize = 2;

    ccc.extend_from_slice(&[
        tlv_tags::DATA_FIELD,
        0x00, // Length - fill this field dynamically at the end.
    ]);

    let mut card_identifier: Vec<u8> = vec![
        tlv_tags::CARD_IDENTIFIER,
        0x00, // Length - fill this field dynamically.
    ];
    // GSC-RID of GSA - TFCS; US.
    card_identifier.extend_from_slice(&[0xa0, 0x00, 0x00, 0x01, 0x16]);
    card_identifier.extend_from_slice(&[
        0xff, // Manufacturer ID - e.g., Yubikey uses 0xff.
        0x02, // Card Type - Java Card.
    ]);
    // CardID - random 14 bytes to fill out the rest of the Card Identifier (21 bytes).
    // Generating a unique one per CCC is more future-proof than hardcoding.
    card_identifier.extend_from_slice(&{
        let mut bytes = [0u8; 14];
        rand::rngs::OsRng.unwrap_err().fill_bytes(&mut bytes);
        bytes
    });
    card_identifier[TLV_L_IDX] = (card_identifier.len() - TLV_TL_SIZE) as u8;

    ccc.extend_from_slice(&card_identifier);
    // Capability Container Version Number (ver. 2.1)
    ccc.extend_from_slice(&[tlv_tags::CAPABILITY_CONTAINER_VERSION_NUMBER, 0x01, 0x21]);
    // Capability Grammar Version Number (ver. 2.1)
    ccc.extend_from_slice(&[tlv_tags::CAPABILITY_GRAMMAR_VERSION_NUMBER, 0x01, 0x21]);
    // Applications CardURL (none)
    ccc.extend_from_slice(&[tlv_tags::APPLICATIONS_CARDURL, 0x00]);
    // PKCS#15 (not supported)
    ccc.extend_from_slice(&[tlv_tags::PKCS15, 0x00]);
    // Registered Data Model Number (GSC-IS, see '8. Data Model': https://nvlpubs.nist.gov/nistpubs/Legacy/IR/nistir6887e2003.pdf#page=135)
    ccc.extend_from_slice(&[tlv_tags::REGISTERED_DATA_MODEL, 0x01, 0x01]);
    // Access Control Rule Table (none)
    ccc.extend_from_slice(&[tlv_tags::ACCESS_CONTROL_RULE_TABLE, 0x00]);
    // Card APDUs (none)
    ccc.extend_from_slice(&[tlv_tags::CARD_APDUS, 0x00]);
    // Redirection Tag (none)
    ccc.extend_from_slice(&[tlv_tags::REDIRECTION_TAG, 0x00]);
    // Capability Tuples (CTs) (none)
    ccc.extend_from_slice(&[tlv_tags::CAPABILITY_TUPLES, 0x00]);
    // Status Tuples (STs) (none)
    ccc.extend_from_slice(&[tlv_tags::STATUS_TUPLES, 0x00]);
    // Next CCC (none)
    ccc.extend_from_slice(&[tlv_tags::NEXT_CCC, 0x00]);
    // Error Detection Code (mandated by GSC-IS)
    ccc.extend_from_slice(&[tlv_tags::ERROR_DETECTION_CODE, 0x00]);

    ccc[TLV_L_IDX] = (ccc.len() - TLV_TL_SIZE) as u8;
    ccc
}
