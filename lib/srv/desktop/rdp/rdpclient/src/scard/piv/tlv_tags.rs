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
// GENERAL AUTHENTICATE command tags.
pub(super) const DYNAMIC_AUTHENTICATION_TEMPLATE: u8 = 0x7C;
pub(super) const CHALLENGE: u8 = 0x81;
pub(super) const RESPONSE: u8 = 0x82;
