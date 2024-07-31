// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

//! This module contains static structures which are used in common by all
//! desktop sessions on a given windows_desktop_service.
//!
//! ## Constraints
//!
//! The primary constraint for maintainers to keep in mind is that these
//! structures can in effect be accessed by multiple threads at any given
//! time via Go, which can call in via [`get_client_handle`] from a goroutine
//! running on any thread, and may call from multiple goroutines (threads)
//! at the same time. Therefore, typical Rust concurrency semantics must be
//! carefully enforced by the programmer (the compiler will not necessarily
//! catch violations that are caused by these structures being called by threads
//! managed by Go).
//!
//! In practice this primarily means ensuring that any such global, static
//! structures that might be accessed directly by a call from go are `Send + Sync`
//! and thus are only mutated when locked. See `assert_send_sync`
//! below for an example of how this is enforced.

use super::ClientHandle;
use crate::CgoHandle;
use parking_lot::RwLock;
use static_init::dynamic;
use std::collections::HashMap;

/// Gets a [`ClientHandle`] from the global [`CLIENT_HANDLES`] map.
pub fn get_client_handle(cgo_handle: CgoHandle) -> Option<ClientHandle> {
    CLIENT_HANDLES.get(cgo_handle)
}

/// A global, static tokio runtime for use by all clients.
#[dynamic]
pub static TOKIO_RT: tokio::runtime::Runtime = tokio::runtime::Runtime::new().unwrap();

/// A global, static map of [`ClientHandle`] indexed by [`CgoHandle`].
///
/// See [`ClientHandles`].
#[dynamic]
pub static CLIENT_HANDLES: ClientHandles = ClientHandles::new();

const _: () = {
    /// References to following types can be shared by multiple
    /// threads (goroutines) simultaneously ([`Sync`]), and Go may
    /// assign these types to be used on any arbitrary thread ([`Send`]),
    /// so we guarantee here that they are [`Send`] + [`Sync`].
    const fn assert_send_sync<T: Send + Sync>() {}
    assert_send_sync::<tokio::runtime::Runtime>();
    assert_send_sync::<ClientHandles>();
};

/// A map of [`ClientHandle`] indexed by [`CgoHandle`].
///
/// A function can be dispatched to the [`Client`] corresponding to a
/// given [`CgoHandle`] by retrieving it's corresponding [`ClientHandle`]
/// from this map and sending the desired [`ClientFunction`].
pub struct ClientHandles {
    map: RwLock<HashMap<CgoHandle, ClientHandle>>,
}

impl ClientHandles {
    fn new() -> Self {
        ClientHandles {
            map: RwLock::new(HashMap::new()),
        }
    }

    pub fn insert(&self, cgo_handle: CgoHandle, client_handle: ClientHandle) {
        self.map.write().insert(cgo_handle, client_handle);
    }

    pub fn get(&self, cgo_handle: CgoHandle) -> Option<ClientHandle> {
        self.map.read().get(&cgo_handle).map(|c| (*c).clone())
    }

    pub fn remove(&self, cgo_handle: CgoHandle) {
        self.map.write().remove(&cgo_handle);
    }
}
