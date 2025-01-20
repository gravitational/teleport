/*
 * *
 *  * Teleport
 *  * Copyright (C) 2024 Gravitational, Inc.
 *  *
 *  * This program is free software: you can redistribute it and/or modify
 *  * it under the terms of the GNU Affero General Public License as published by
 *  * the Free Software Foundation, either version 3 of the License, or
 *  * (at your option) any later version.
 *  *
 *  * This program is distributed in the hope that it will be useful,
 *  * but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  * GNU Affero General Public License for more details.
 *  *
 *  * You should have received a copy of the GNU Affero General Public License
 *  * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */
use crate::{
    cgo_free_rdp_license, cgo_read_rdp_license, cgo_write_rdp_license, CGOErrCode,
    CGOLicenseRequest,
};
use ironrdp_connector::{ConnectorError, ConnectorErrorExt, ConnectorResult, LicenseCache};
use ironrdp_pdu::rdp::server_license::LicenseInformation;
use picky_krb::negoex::NegoexDataType;
use std::{ptr, slice};

#[derive(Debug)]
pub(crate) struct GoLicenseCache {}

impl LicenseCache for GoLicenseCache {
    fn get_license(&self, license_info: LicenseInformation) -> ConnectorResult<Option<Vec<u8>>> {
        let mut req = CGOLicenseRequest {
            version: license_info.version,
            issuer: license_info.scope.into(),
            company: license_info.company_name.into(),
            product_id: license_info.product_id.into(),
        };
        let mut data: *mut u8 = ptr::null_mut();
        let mut size = 0usize;
        unsafe {
            match cgo_read_rdp_license((), &mut req, &mut data, &mut size) {
                CGOErrCode::ErrCodeSuccess => {
                    let license = slice::from_raw_parts_mut(data, size).to_vec();
                    cgo_free_rdp_license(data);
                    Ok(Some(license))
                }
                CGOErrCode::ErrCodeFailure => Err(ConnectorError::general("")),
                CGOErrCode::ErrCodeClientPtr => Err(ConnectorError::general("")),
                CGOErrCode::ErrCodeNotFound => Ok(None),
            }
        }
    }

    fn store_license(&self, mut license_info: LicenseInformation) -> ConnectorResult<()> {
        let mut req = CGOLicenseRequest {
            version: license_info.version,
            issuer: license_info.scope.into(),
            company: license_info.company_name.into(),
            product_id: license_info.product_id.into(),
        };
        unsafe {
            match cgo_write_rdp_license(
                (),
                &mut req,
                license_info.license_info.as_mut_ptr(),
                license_info.license_info.size(),
            ) {
                CGOErrCode::ErrCodeSuccess => Ok(()),
                _ => Err(ConnectorError::general("")),
            }
        }
    }
}
