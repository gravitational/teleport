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

use crate::ipc::IpcClient;
use ironrdp_connector::{custom_err, ConnectorResult, LicenseCache};
use ironrdp_pdu::rdp::server_license::LicenseInformation;
use rdp_client_proto::desktop;
use std::sync::Mutex;

#[derive(Debug)]
pub(crate) struct GoLicenseCache {
    pub(crate) ipc_client: Mutex<IpcClient>,
}

impl LicenseCache for GoLicenseCache {
    fn get_license(&self, license_info: LicenseInformation) -> ConnectorResult<Option<Vec<u8>>> {
        let mut ipc_client = self
            .ipc_client
            .lock()
            .expect("license cache mutex poisoned");

        let response = tokio::runtime::Handle::current()
            .block_on(ipc_client.read_rdp_license(desktop::LicenseMetadata {
                version: license_info.version,
                issuer: license_info.scope,
                company: license_info.company_name,
                product_id: license_info.product_id,
            }))
            .map_err(|e| custom_err!("error retrieving license", e))?;

        Ok(response.into_inner().license_info)
    }

    fn store_license(&self, license_info: LicenseInformation) -> ConnectorResult<()> {
        let mut ipc_client = self
            .ipc_client
            .lock()
            .expect("license cache mutex poisoned");

        tokio::runtime::Handle::current()
            .block_on(ipc_client.write_rdp_license(desktop::License {
                metadata: Some(desktop::LicenseMetadata {
                    version: license_info.version,
                    issuer: license_info.scope,
                    company: license_info.company_name,
                    product_id: license_info.product_id,
                }),
                license_info: license_info.license_info,
            }))
            .map_err(|e| custom_err!("error storing license", e))?;

        Ok(())
    }
}
