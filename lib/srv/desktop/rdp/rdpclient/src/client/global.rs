//! This module contains static structures which are used in common by all
//! desktop sessions. It provides a public API for safely accessing these
//! structures given this constraint.
//!
//! ## Constraints
//!
//! The primary constraint for maintainers to keep in mind is that these
//! structures can in effect be accessed by multiple threads at any given
//! time via Go. Therefore, typical Rust concurrency semantics must be
//! carefully observed (the compiler will not necessarily catch violations).
//!
//! In practice this primarily means ensuring that such global, static
//! structures are [`Send`] + [`Sync`], and that only methods which deal
//! with immutable references to them are exposed via the public API.

use super::{Client, ClientFunction};
use crate::{CGOErrCode, CgoHandle};
use futures_util::Future;
use parking_lot::RwLock;
use static_init::dynamic;
use std::collections::HashMap;
use tokio::sync::mpsc::{channel, Receiver, Sender};
use tokio::task::JoinHandle;

pub fn tokio_block_on<F: Future>(future: F) -> F::Output {
    TOKIO_RT.block_on(future)
}

pub fn tokio_spawn<F>(future: F) -> JoinHandle<F::Output>
where
    F: Future + Send + 'static,
    F::Output: Send + 'static,
{
    TOKIO_RT.spawn(future)
}

pub fn register(client: &mut Client) -> FunctionReceiver {
    let (client_handle, function_receiver) = channel(100);
    CLIENT_HANDLES.insert(client.cgo_handle, client_handle);
    function_receiver
}

pub fn deregister(cgo_handle: CgoHandle) {
    CLIENT_HANDLES.remove(cgo_handle)
}

/// Calls a function on a client handle.
///
/// This function takes a [`CgoHandle`] and a [`ClientFunction`] as parameters.
/// It attempts to get the client handle from the global [`CLIENT_HANDLES`] map.
/// If the handle is found, it tries to send the function to the client handle.
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
            Err(_) => return CGOErrCode::ErrCodeFailure,
        }
    }

    CGOErrCode::ErrCodeFailure
}

/// A global, static tokio runtime for use by all clients.
#[dynamic]
static TOKIO_RT: tokio::runtime::Runtime = tokio::runtime::Runtime::new().unwrap();

/// A global, static map of [`ClientHandle`] indexed by [`CgoHandle`].
///
/// See [`ClientHandles`].
#[dynamic]
static CLIENT_HANDLES: ClientHandles = ClientHandles::new();

const _: () = {
    const fn assert_send_sync<T: Send + Sync>() {}
    // Immutable references to following types can be used directly by multiple
    // threads (goroutines) simultaneously, so we guarantee that they are Send + Sync.
    assert_send_sync::<tokio::runtime::Runtime>();
    assert_send_sync::<ClientHandles>();
};

/// A map of [`ClientHandle`] indexed by [`CgoHandle`].
///
/// A function can be dispatched to the [`Client`] corresponding to a
/// given [`CgoHandle`] by retrieving it from this map and sending
/// the desired function and arguments to the [`ClientFunctionDispatcher`].
struct ClientHandles {
    map: RwLock<HashMap<CgoHandle, ClientHandle>>,
}

impl ClientHandles {
    fn new() -> Self {
        ClientHandles {
            map: RwLock::new(HashMap::new()),
        }
    }

    fn insert(&self, cgo_handle: CgoHandle, dispatcher: ClientHandle) {
        self.map.write().insert(cgo_handle, dispatcher);
    }

    pub fn get(&self, cgo_handle: CgoHandle) -> Option<ClientHandle> {
        self.map.read().get(&cgo_handle).map(|c| (*c).clone())
    }

    fn remove(&self, cgo_handle: CgoHandle) {
        self.map.write().remove(&cgo_handle);
    }
}

/// `ClientHandle` is a type alias for a `Sender` that sends `ClientFunction` enums.
///
/// It is used to dispatch [`ClientFunction`]s to a corresponding [`FunctionReceiver`]
/// on a `Client`.
type ClientHandle = Sender<ClientFunction>;

/// `FunctionReceiver` is a type alias for a `Receiver` that receives [`ClientFunction`]
/// enums.
///
/// Each `Client` has a `FunctionReceiver` that it listens to for [`ClientFunction`]
/// requests sent by its corresponding [`ClientHandle`], and subseqently calls a corresponding
/// function whenever such requests are received.
pub type FunctionReceiver = Receiver<ClientFunction>;
