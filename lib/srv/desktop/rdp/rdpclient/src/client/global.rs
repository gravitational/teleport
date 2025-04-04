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
use std::{
    collections::HashMap,
    sync::{LazyLock, Mutex},
};

/// Gets a [`ClientHandle`] from the global [`CLIENT_HANDLES`] map.
pub fn get_client_handle(cgo_handle: CgoHandle) -> Option<ClientHandle> {
    CLIENT_HANDLES.get(cgo_handle)
}

/// A global, static map of [`ClientHandle`] indexed by [`CgoHandle`].
///
/// See [`ClientHandles`].
pub static CLIENT_HANDLES: LazyLock<ClientHandles> = LazyLock::new(Default::default);

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
/// given [`CgoHandle`] by retrieving its corresponding [`ClientHandle`]
/// from this map and sending the desired [`ClientFunction`].
#[derive(Default)]
pub struct ClientHandles {
    map: Mutex<HashMap<CgoHandle, ClientHandle>>,
}

impl ClientHandles {
    pub fn new() -> Self {
        Default::default()
    }

    pub fn insert(&self, cgo_handle: CgoHandle, client_handle: ClientHandle) {
        self.map.lock().unwrap().insert(cgo_handle, client_handle);
    }

    pub fn get(&self, cgo_handle: CgoHandle) -> Option<ClientHandle> {
        self.map.lock().unwrap().get(&cgo_handle).cloned()
    }

    pub fn remove(&self, cgo_handle: CgoHandle) {
        self.map.lock().unwrap().remove(&cgo_handle);
    }
}
