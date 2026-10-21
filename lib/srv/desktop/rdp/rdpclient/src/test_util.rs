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

use rsa::pkcs1::DecodeRsaPrivateKey;
use rsa::RsaPrivateKey;
use std::sync::{Arc, LazyLock};

pub const RSA_2048_DER: &[u8] = include_bytes!("../testdata/rsa2048.der");

static RSA_2048_KEY: LazyLock<Arc<RsaPrivateKey>> = LazyLock::new(|| {
    Arc::new(RsaPrivateKey::from_pkcs1_der(RSA_2048_DER).expect("failed to parse RSA 2048 key"))
});

pub fn rsa_2048_key() -> Arc<RsaPrivateKey> {
    RSA_2048_KEY.clone()
}
