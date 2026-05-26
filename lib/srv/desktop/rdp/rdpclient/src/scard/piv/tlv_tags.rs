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

// SELECT command tags.
pub(super) const PIV_APPLICATION_PROPERTY_TEMPLATE: u8 = 0x61;
pub(super) const AID: u8 = 0x4F;
pub(super) const COEXISTENT_TAG_ALLOCATION_AUTHORITY: u8 = 0x79;
pub(super) const DATA_FIELD: u8 = 0x53;
pub(super) const FASC_N: u8 = 0x30;
pub(super) const GUID: u8 = 0x34;
pub(super) const EXPIRATION_DATE: u8 = 0x35;
pub(super) const ISSUER_ASYMMETRIC_SIGNATURE: u8 = 0x3E;
pub(super) const ERROR_DETECTION_CODE: u8 = 0xFE;
pub(super) const CERTIFICATE: u8 = 0x70;
pub(super) const CERTINFO: u8 = 0x71;
pub(super) const CARD_IDENTIFIER: u8 = 0xF0;
pub(super) const CAPABILITY_CONTAINER_VERSION_NUMBER: u8 = 0xF1;
pub(super) const CAPABILITY_GRAMMAR_VERSION_NUMBER: u8 = 0xF2;
pub(super) const APPLICATIONS_CARDURL: u8 = 0xF3;
pub(super) const PKCS15: u8 = 0xF4;
pub(super) const REGISTERED_DATA_MODEL: u8 = 0xF5;
pub(super) const ACCESS_CONTROL_RULE_TABLE: u8 = 0xF6;
pub(super) const CARD_APDUS: u8 = 0xF7;
pub(super) const REDIRECTION_TAG: u8 = 0xFA;
pub(super) const CAPABILITY_TUPLES: u8 = 0xFB;
pub(super) const STATUS_TUPLES: u8 = 0xFC;
pub(super) const NEXT_CCC: u8 = 0xFD;
// GENERAL AUTHENTICATE command tags.
pub(super) const DYNAMIC_AUTHENTICATION_TEMPLATE: u8 = 0x7C;
pub(super) const CHALLENGE: u8 = 0x81;
pub(super) const RESPONSE: u8 = 0x82;
