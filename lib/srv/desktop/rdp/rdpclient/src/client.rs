// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

use std::{io, ops::Deref};

use bytes::BytesMut;
use ironrdp_session::{x224, ActiveStageOutput, SessionResult};
use tokio::net::TcpStream as TokioTcpStream;
use tokio::{runtime::Runtime, sync::Mutex};

use crate::{assert_impl_all, CGODisconnectCode, CGOErrCode, ReadRdpOutputReturns};

/// FFIClient is a wrapper around a pointer to a [`RustClient`] which is built
/// to be passed to and from Go. FFIClient implements [`Deref`] to &[`RustClient`],
/// which means that it can be used directly as a &RustClient. See the public
/// methods of [`RustClient`] for methods which can be called on it.
///
/// # Usage
///
/// After calling [`FFIClient::new()`], the caller is responsible for calling [`FFIClient::drop()`].
/// Letting the FFIClient go out of scope without calling [`FFIClient::drop()`] will result in a memory
/// leak.
///
/// # Safety
///
/// Using the FFIClient after calling [`FFIClient::drop()`], calling [`FFIClient::drop()`] twice on the same FFIClient,
/// or calling [`FFIClient::drop()`] on a FFIClient which was created with [`FFIClient::new_null()`] will all result
/// in undefined behavior.
///
/// # Thread Safety
///
/// FFIClient is built to be passed to and from Go, which can use it to call back into Rust
/// from arbitrary goroutines. This means that it must be [`Send`] + [`Sync`], because
/// goroutines might be scheduled on different threads and can run concurrently.
///
/// In order to ensure that FFIClient is [`Send`] + [`Sync`], FFIClient's only safe methods
/// are those which are accessible via its ability to [`Deref`] to an &[`RustClient`]. This
/// means that FFIClient is effectively a proxy to an &[`RustClient`], and thus is [`Send`] + [`Sync`]
/// insofar as &[`RustClient`] is [`Send`] + [`Sync`].
///
/// We know &[`RustClient`] is [`Send`] + [`Sync`] because we enforce that [`RustClient`] is
/// [`Send`] + [`Sync`] (via the `assert_impl_all!(RustClient: Send, Sync)` macro). Therefore,
/// because ["&T is Send if and only if T is Sync"](https://doc.rust-lang.org/std/marker/trait.Sync.html)
/// and ["&T and &mut T are Sync if and only if T is Sync"](https://doc.rust-lang.org/std/marker/trait.Sync.html),
/// we know that FFIClient is [`Send`] + [`Sync`].
#[repr(C)]
pub struct FFIClient(*const RustClient);
impl FFIClient {
    /// Creates a new [`FFIClient`]. The caller MUST call
    /// [`FFIClient::drop()`] on the returned [`FFIClient`]
    /// when they are done with it, or it will result
    /// in a memory leak.
    pub fn new(iron_rdp_client: IronRDPClient, go_ref: usize, tokio_rt: Runtime) -> Self {
        Self(Box::into_raw(Box::new(RustClient::new(
            iron_rdp_client,
            go_ref,
            tokio_rt,
        ))))
    }

    /// Creates a new null [`FFIClient`]. The caller MUST not call
    /// [`FFIClient::drop()`] on a null [`FFIClient`] or it will result
    /// in undefined behavior.
    pub fn new_null() -> Self {
        Self(std::ptr::null())
    }

    /// # Safety
    ///
    /// This function is unsafe because improper use may lead to
    /// memory problems. For example, a double-free may occur if the
    /// function is called twice on the same FFIClient (created with
    /// FFIClient::new), or if it is called on a FFIClient created with
    /// FFIClient::new_null.
    ///
    /// While Rust's memory semantics make this impossible in pure Rust,
    /// it is possible to call this function twice from Go.
    pub unsafe fn drop(self) {
        drop(Box::from_raw(self.0 as *mut RustClient));
    }
}

/// # Safety
///
/// See "Thread Safety" in [`FFIClient`] for more details.
unsafe impl Send for FFIClient {}
/// # Safety
///
/// See "Thread Safety" in [`FFIClient`] for more details.
unsafe impl Sync for FFIClient {}

impl Deref for FFIClient {
    type Target = RustClient;

    #[inline]
    fn deref(&self) -> &Self::Target {
        // Safety:
        //
        // Panics if the pointer is null.
        unsafe {
            if let Some(c) = self.0.as_ref() {
                c
            } else {
                panic!("attempted to dereference a null FFIClient");
            }
        }
    }
}

/// RustClient must be Send + Sync, see [`FFIClient`] for more details.
pub struct RustClient {
    iron_rdp_client: Mutex<IronRDPClient>,
    pub tokio_rt: Runtime,
    pub go_ref: usize,
}

// Forces RustClient to be Send + Sync
assert_impl_all!(RustClient: Send, Sync);

impl RustClient {
    fn new(iron_rdp_client: IronRDPClient, go_ref: usize, tokio_rt: Runtime) -> Self {
        Self {
            iron_rdp_client: Mutex::new(iron_rdp_client),
            tokio_rt,
            go_ref,
        }
    }

    pub async fn read_pdu(&self) -> io::Result<(ironrdp_pdu::Action, BytesMut)> {
        self.iron_rdp_client.lock().await.framed.read_pdu().await
    }

    pub async fn write_all(&self, buf: &[u8]) -> io::Result<()> {
        self.iron_rdp_client
            .lock()
            .await
            .framed
            .write_all(buf)
            .await
    }

    pub async fn process_x224_frame(&self, frame: &[u8]) -> SessionResult<Vec<ActiveStageOutput>> {
        let output = self
            .iron_rdp_client
            .lock()
            .await
            .x224_processor
            .process(frame)?;
        let mut stage_outputs = Vec::new();
        if !output.is_empty() {
            stage_outputs.push(ActiveStageOutput::ResponseFrame(output));
        }
        Ok(stage_outputs)
    }

    /// Iterates through any response frames in result, sending them to the RDP server.
    /// Typically returns None if everything goes as expected and the session should continue.
    // TODO(isaiah): this api is weird, should probably return a Result instead of an Option.
    pub async fn process_active_stage_result(
        &self,
        result: SessionResult<Vec<ActiveStageOutput>>,
    ) -> Option<ReadRdpOutputReturns> {
        match result {
            Ok(outputs) => {
                for output in outputs {
                    match output {
                        ActiveStageOutput::ResponseFrame(response) => {
                            match self.write_all(&response).await {
                                Ok(_) => {
                                    trace!("write_all succeeded, continuing");
                                    continue;
                                }
                                Err(e) => {
                                    return Some(ReadRdpOutputReturns {
                                        user_message: format!("Failed to write frame: {}", e),
                                        disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                                        err_code: CGOErrCode::ErrCodeFailure,
                                    });
                                }
                            }
                        }
                        ActiveStageOutput::Terminate => {
                            return Some(ReadRdpOutputReturns {
                                user_message: "RDP session terminated".to_string(),
                                disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                                err_code: CGOErrCode::ErrCodeSuccess,
                            });
                        }
                        ActiveStageOutput::GraphicsUpdate(_) => {
                            error!("unexpected GraphicsUpdate, this should be handled on the client side");
                            return Some(ReadRdpOutputReturns {
                                user_message: "Server error".to_string(),
                                disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                                err_code: CGOErrCode::ErrCodeFailure,
                            });
                        }
                    }
                }
            }
            Err(err) => {
                error!("failed to process frame: {}", err);
                return Some(ReadRdpOutputReturns {
                    user_message: "Failed to process frame".to_string(),
                    disconnect_code: CGODisconnectCode::DisconnectCodeUnknown,
                    err_code: CGOErrCode::ErrCodeFailure,
                });
            }
        }

        // All outputs were response frames, return None to indicate that the client should continue
        trace!("process_active_stage_result succeeded, returning None");
        None
    }
}

type UpgradedFramed = ironrdp_tokio::TokioFramed<ironrdp_tls::TlsStream<TokioTcpStream>>;

pub struct IronRDPClient {
    framed: UpgradedFramed,
    x224_processor: x224::Processor,
}

impl IronRDPClient {
    pub fn new(upgraded_framed: UpgradedFramed, x224_processor: x224::Processor) -> Self {
        Self {
            framed: upgraded_framed,
            x224_processor,
        }
    }
}
