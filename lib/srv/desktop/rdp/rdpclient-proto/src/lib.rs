// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

pub mod tdpb;

pub mod desktop {
    pub(super) mod v1 {
        tonic::include_proto!("teleport.desktop.v1");
    }

    pub use v1::{
        desktop_service_client::DesktopServiceClient, CertificateAndKey, License, LicenseMetadata,
    };
}

mod mfa {
    pub mod v2 {
        tonic::include_proto!("teleport.mfa.v2");
    }
}

mod webauthn {
    pub mod v2 {
        tonic::include_proto!("teleport.webauthn.v2");
    }
}
