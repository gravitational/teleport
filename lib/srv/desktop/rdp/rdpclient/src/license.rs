// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

use crate::{
    cgo_free_rdp_license, cgo_read_rdp_license, cgo_write_rdp_license, CGOErrCode,
    CGOLicenseRequest, CgoHandle,
};
use ironrdp_connector::{custom_err, general_err, ConnectorError, ConnectorResult, LicenseCache};
use ironrdp_pdu::rdp::server_license::LicenseInformation;
use std::ffi::{CString, NulError};
use std::{ptr, slice};

#[derive(Debug)]
pub(crate) struct GoLicenseCache {
    pub(crate) cgo_handle: CgoHandle,
}

fn conversion_error(e: NulError) -> ConnectorError {
    custom_err!("conversion error", e)
}

impl LicenseCache for GoLicenseCache {
    fn get_license(&self, license_info: LicenseInformation) -> ConnectorResult<Option<Vec<u8>>> {
        let issuer = CString::new(license_info.scope).map_err(conversion_error)?;
        let company = CString::new(license_info.company_name).map_err(conversion_error)?;
        let product_id = CString::new(license_info.product_id).map_err(conversion_error)?;
        let mut req = CGOLicenseRequest {
            version: license_info.version,
            issuer: issuer.as_ptr(),
            company: company.as_ptr(),
            product_id: product_id.as_ptr(),
        };
        let mut data: *mut u8 = ptr::null_mut();
        let mut size = 0usize;
        unsafe {
            match cgo_read_rdp_license(self.cgo_handle, &mut req, &mut data, &mut size) {
                CGOErrCode::ErrCodeSuccess => {
                    let license = slice::from_raw_parts_mut(data, size).to_vec();
                    cgo_free_rdp_license(data);
                    Ok(Some(license))
                }
                CGOErrCode::ErrCodeFailure => Err(general_err!("error retrieving license")),
                CGOErrCode::ErrCodeClientPtr => Err(general_err!("invalid client pointer")),
                CGOErrCode::ErrCodeNotFound => Ok(None),
            }
        }
    }

    fn store_license(&self, mut license_info: LicenseInformation) -> ConnectorResult<()> {
        let issuer = CString::new(license_info.scope).map_err(conversion_error)?;
        let company = CString::new(license_info.company_name).map_err(conversion_error)?;
        let product_id = CString::new(license_info.product_id).map_err(conversion_error)?;
        let mut req = CGOLicenseRequest {
            version: license_info.version,
            issuer: issuer.as_ptr(),
            company: company.as_ptr(),
            product_id: product_id.as_ptr(),
        };
        unsafe {
            match cgo_write_rdp_license(
                self.cgo_handle,
                &mut req,
                license_info.license_info.as_mut_ptr(),
                license_info.license_info.len(),
            ) {
                CGOErrCode::ErrCodeSuccess => Ok(()),
                _ => Err(general_err!("error storing license")),
            }
        }
    }
}
