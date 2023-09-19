//! This module contains static structures which are used in common by all
//! desktop sessions on a given windows_desktop_service.
//!
//! ## Constraints
//!
//! The primary constraint for maintainers to keep in mind is that these
//! structures can in effect be accessed by multiple threads at any given
//! time via Go. Therefore, typical Rust concurrency semantics must be
//! carefully observed (the compiler will not necessarily catch violations).
//!
//! In practice this primarily means ensuring that such global, static
//! structures are [`Send`] + [`Sync`] and are only mutated when locked.

use super::{ClientFunction, ClientHandle};
use crate::{CGOErrCode, CgoHandle};
use parking_lot::RwLock;
use static_init::dynamic;
use std::collections::HashMap;

/// Calls a function on a client handle.
///
/// This function takes a [`CgoHandle`] and a [`ClientFunction`] as parameters.
/// It attempts to get the [`ClientHandle`] from those registered in the global [`CLIENT_HANDLES`] map.
/// If the handle is found, it tries to send the [`ClientFunction`] to the [`Client`] via the [`ClientHandle`].
/// If the function is successfully sent, it returns [`CGOErrCode::ErrCodeSuccess`].
/// If the function fails to send, it returns [`CGOErrCode::ErrCodeFailure`].
/// If the handle is not found in the [`CLIENT_HANDLES`] map, it also returns [`CGOErrCode::ErrCodeFailure`].
///
/// Note that the return value does not represent the result of the function call itself,
/// rather it represents whether or not request to call a function was successfully sent over
/// a channel. Error handling for the function call itself is handled by [`Client`].
pub fn call_function_on_handle(cgo_handle: CgoHandle, func: ClientFunction) -> CGOErrCode {
    if let Some(handle) = CLIENT_HANDLES.get(cgo_handle) {
        match handle.blocking_send(func) {
            Ok(_) => return CGOErrCode::ErrCodeSuccess,
            Err(e) => {
                warn!("call_function_on_handle failed: {}", e);
                return CGOErrCode::ErrCodeFailure;
            }
        }
    }

    CGOErrCode::ErrCodeFailure
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
    /// Immutable references to following types can be used directly by multiple
    /// threads (goroutines) simultaneously, so we guarantee here that they are Send.
    ///
    /// These must be Sync as well, however this is already guaranteed by the compiler's
    /// constraints for `static` variables. (See https://doc.rust-lang.org/reference/items/static-items.html)
    const fn assert_send<T: Send>() {}
    assert_send::<tokio::runtime::Runtime>();
    assert_send::<ClientHandles>();
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
