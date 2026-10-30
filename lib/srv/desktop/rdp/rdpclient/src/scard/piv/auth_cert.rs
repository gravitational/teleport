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

use crate::scard::piv::{tlv_tags, utils};

pub(super) fn build_piv_auth_cert(cert_der: &[u8]) -> Vec<u8> {
    // Tags in this BER-TLV value are not compatible with the spec
    // and existing libraries. Marshal by hand.
    //
    // Certificate TLV tag and length.
    let mut resp = vec![tlv_tags::CERTIFICATE];
    resp.extend_from_slice(&utils::len_to_vec(cert_der.len()));
    resp.extend_from_slice(cert_der);
    // CertInfo TLV (0x00 indicates uncompressed cert).
    resp.extend_from_slice(&[tlv_tags::CERTINFO, 0x01, 0x00]);
    // TLV for error detection code.
    resp.extend_from_slice(&[tlv_tags::ERROR_DETECTION_CODE, 0x00]);

    // Wrap with top-level TLV tag and length.
    let mut resp_outer = vec![tlv_tags::DATA_FIELD];
    resp_outer.extend_from_slice(&utils::len_to_vec(resp.len()));
    resp_outer.extend_from_slice(&resp);
    resp_outer
}
