pub mod errors;
pub mod rdpdr;
pub mod scard;

#[macro_use]
extern crate log;
#[macro_use]
extern crate num_derive;
extern crate byteorder;

use libc::{fd_set, select, FD_SET};
use rdp::core::event::*;
use rdp::core::gcc::KeyboardLayout;
use rdp::core::global;
use rdp::core::mcs;
use rdp::core::sec;
use rdp::core::tpkt;
use rdp::core::x224;
use rdp::model::error::{Error as RdpError, RdpError as RdpProtocolError, RdpErrorKind, RdpResult};
use rdp::model::link::{Link, Stream};
use rdp::nla::ntlm::Ntlm;
use std::convert::TryFrom;
use std::ffi::{CStr, CString};
use std::io::Error as IoError;
use std::io::{Cursor, Read, Write};
use std::mem;
use std::net::TcpStream;
use std::os::raw::c_char;
use std::os::unix::io::AsRawFd;
use std::ptr;
use std::sync::{Arc, Mutex};

#[no_mangle]
pub extern "C" fn init() {
    env_logger::try_init().unwrap_or_else(|e| println!("failed to initialize Rust logger: {}", e));
}

// Client has an unusual lifecycle:
// - connect_rdp creates it on the heap, grabs a raw pointer and returns in to Go
// - most other exported rdp functions take the raw pointer, convert it to a reference for use
//   without dropping the Client
// - close_rdp takes the raw pointer and drops it
//
// All of the exported rdp functions could run concurrently, so the rdp_client is synchronized.
// tcp_fd is only set in connect_rdp and used as read-only afterwards, so it does not need
// synchronization.
pub struct Client {
    rdp_client: Arc<Mutex<RdpClient<TcpStream>>>,
    tcp_fd: usize,
}

impl Client {
    fn into_raw(self: Box<Self>) -> *mut Self {
        Box::into_raw(self)
    }
    unsafe fn from_ptr<'a>(ptr: *const Self) -> Option<&'a Client> {
        ptr.as_ref()
    }
    unsafe fn from_raw(ptr: *mut Self) -> Box<Self> {
        Box::from_raw(ptr)
    }
}

#[repr(C)]
pub struct ClientOrError {
    client: *mut Client,
    err: CGOError,
}

impl From<Result<Client, ConnectError>> for ClientOrError {
    fn from(r: Result<Client, ConnectError>) -> ClientOrError {
        match r {
            Ok(client) => ClientOrError {
                client: Box::new(client).into_raw(),
                err: CGO_OK,
            },
            Err(e) => ClientOrError {
                client: ptr::null_mut(),
                err: to_cgo_error(format!("{:?}", e)),
            },
        }
    }
}

// connect_rdp establishes an RDP connection to go_addr with the provided credentials and screen
// size. If succeeded, the client is internally registered under client_ref. When done with the
// connection, the caller must call close_rdp.
#[no_mangle]
pub extern "C" fn connect_rdp(
    go_addr: *mut c_char,
    go_username: *mut c_char,
    go_password: *mut c_char,
    screen_width: u16,
    screen_height: u16,
) -> ClientOrError {
    // Convert from C to Rust types.
    let addr = from_go_string(go_addr);
    let username = from_go_string(go_username);
    let password = from_go_string(go_password);

    connect_rdp_inner(&addr, username, password, screen_width, screen_height).into()
}

#[derive(Debug)]
enum ConnectError {
    TCP(IoError),
    RDP(RdpError),
}

impl From<IoError> for ConnectError {
    fn from(e: IoError) -> ConnectError {
        ConnectError::TCP(e)
    }
}

impl From<RdpError> for ConnectError {
    fn from(e: RdpError) -> ConnectError {
        ConnectError::RDP(e)
    }
}

fn connect_rdp_inner(
    addr: &str,
    username: String,
    password: String,
    screen_width: u16,
    screen_height: u16,
) -> Result<Client, ConnectError> {
    // Connect and authenticate.
    let tcp = TcpStream::connect(addr)?;
    let tcp_fd = tcp.as_raw_fd() as usize;
    let domain = ".";

    // From rdp-rs/src/core/client.rs
    let tcp = Link::new(Stream::Raw(tcp));
    let mut authentication = Ntlm::new(domain.to_string(), username.clone(), password.clone());
    let protocols = x224::Protocols::ProtocolSSL as u32 | x224::Protocols::ProtocolHybrid as u32;
    let x224 = x224::Client::connect(
        tpkt::Client::new(tcp),
        protocols,
        false,
        Some(&mut authentication),
        false,
        false,
    )?;
    let mut mcs = mcs::Client::new(x224);
    mcs.connect(
        "rdp-rs".to_string(),
        screen_width,
        screen_height,
        KeyboardLayout::US,
        &vec!["rdpdr".to_string(), "rdpsnd".to_string()],
    )?;
    sec::connect(&mut mcs, &domain.to_string(), &username, &password, false)?;
    let global = global::Client::new(
        mcs.get_user_id(),
        mcs.get_global_channel_id(),
        screen_width,
        screen_height,
        KeyboardLayout::US,
        "rdp-rs",
    );
    let rdpdr = rdpdr::Client::new();

    let rdp_client = RdpClient { mcs, global, rdpdr };
    Ok(Client {
        rdp_client: Arc::new(Mutex::new(rdp_client)),
        tcp_fd: tcp_fd,
    })
}

// From rdp-rs/src/core/client.rs
struct RdpClient<S> {
    mcs: mcs::Client<S>,
    global: global::Client,
    rdpdr: rdpdr::Client,
}

impl<S: Read + Write> RdpClient<S> {
    pub fn read<T>(&mut self, callback: T) -> RdpResult<()>
    where
        T: FnMut(RdpEvent),
    {
        let (channel_name, message) = self.mcs.read()?;
        match channel_name.as_str() {
            "global" => self.global.read(message, &mut self.mcs, callback),
            "rdpdr" => self.rdpdr.read(message, &mut self.mcs),
            _ => Err(RdpError::RdpError(RdpProtocolError::new(
                RdpErrorKind::UnexpectedType,
                &format!("Invalid channel name {:?}", channel_name),
            ))),
        }
    }
    pub fn write(&mut self, event: RdpEvent) -> RdpResult<()> {
        match event {
            RdpEvent::Pointer(pointer) => {
                self.global.write_input_event(pointer.into(), &mut self.mcs)
            }
            RdpEvent::Key(key) => self.global.write_input_event(key.into(), &mut self.mcs),
            _ => Err(RdpError::RdpError(RdpProtocolError::new(
                RdpErrorKind::UnexpectedType,
                "RDPCLIENT: This event can't be sent",
            ))),
        }
    }
    pub fn shutdown(&mut self) -> RdpResult<()> {
        self.mcs.shutdown()
    }
}

#[repr(C)]
pub struct CGOBitmap {
    pub dest_left: u16,
    pub dest_top: u16,
    pub dest_right: u16,
    pub dest_bottom: u16,
    // Memory is freed on the Rust side.
    pub data_ptr: *mut u8,
    pub data_len: usize,
    pub data_cap: usize,
}

impl TryFrom<BitmapEvent> for CGOBitmap {
    type Error = RdpError;

    fn try_from(e: BitmapEvent) -> Result<Self, Self::Error> {
        let mut res = CGOBitmap {
            dest_left: e.dest_left,
            dest_top: e.dest_top,
            dest_right: e.dest_right,
            dest_bottom: e.dest_bottom,
            data_ptr: ptr::null_mut(),
            data_len: 0,
            data_cap: 0,
        };

        // e.decompress consumes e, so we need to call it separately, after populating the fields
        // above.
        let mut data = if e.is_compress {
            e.decompress()?
        } else {
            e.data
        };
        res.data_ptr = data.as_mut_ptr();
        res.data_len = data.len();
        res.data_cap = data.capacity();
        mem::forget(data);

        Ok(res)
    }
}

impl Drop for CGOBitmap {
    fn drop(&mut self) {
        // Reconstruct into Vec to drop the allocated buffer.
        unsafe {
            let _ = Vec::from_raw_parts(self.data_ptr, self.data_len, self.data_cap);
        }
    }
}

#[cfg(unix)]
fn wait_for_fd(fd: usize) -> bool {
    unsafe {
        let mut raw_fds: fd_set = mem::zeroed();

        FD_SET(fd as i32, &mut raw_fds);

        let result = select(
            fd as i32 + 1,
            &mut raw_fds,
            ptr::null_mut(),
            ptr::null_mut(),
            ptr::null_mut(),
        );
        result == 1
    }
}

// read_rdp_output reads incoming RDP bitmap frames from client at client_ref and forwards them to
// handle_bitmap. handle_bitmap *must not* free the memory of CGOBitmap.
#[no_mangle]
pub extern "C" fn read_rdp_output(client_ptr: *mut Client, client_ref: i64) -> CGOError {
    let client = unsafe { Client::from_ptr(client_ptr) };
    let client = match client {
        Some(client) => client,
        None => {
            return to_cgo_error("invalid Rust client pointer".to_string());
        }
    };
    if let Some(err) = read_rdp_output_inner(client, client_ref) {
        to_cgo_error(err)
    } else {
        CGO_OK
    }
}

fn read_rdp_output_inner(client: &Client, client_ref: i64) -> Option<String> {
    let tcp_fd = client.tcp_fd;
    // Read incoming events.
    while wait_for_fd(tcp_fd as usize) {
        let mut err = CGO_OK;
        let res = client
            .rdp_client
            .lock()
            .unwrap()
            .read(|rdp_event| match rdp_event {
                RdpEvent::Bitmap(bitmap) => {
                    let cbitmap = match CGOBitmap::try_from(bitmap) {
                        Ok(cb) => cb,
                        Err(e) => {
                            error!(
                                "failed to convert RDP bitmap to CGO representation: {:?}",
                                e
                            );
                            return;
                        }
                    };
                    unsafe {
                        err = handle_bitmap(client_ref, cbitmap) as CGOError;
                    };
                }
                // These should never really be sent by the server to us.
                RdpEvent::Pointer(_) => {
                    debug!("got unexpected pointer event from RDP server, ignoring");
                }
                RdpEvent::Key(_) => {
                    debug!("got unexpected keyboard event from RDP server, ignoring");
                }
            });
        if let Err(e) = res {
            return Some(format!("failed forwarding RDP bitmap frame: {:?}", e));
        };
        if err != CGO_OK {
            let err_str = from_cgo_error(err);
            return Some(format!("failed forwarding RDP bitmap frame: {}", err_str));
        }
    }
    None
}

// A CGO-compatible copy of PointerEvent.
#[repr(C)]
#[derive(Copy, Clone)]
pub struct CGOPointer {
    pub x: u16,
    pub y: u16,
    pub button: CGOPointerButton,
    pub down: bool,
}

#[repr(C)]
#[derive(Copy, Clone)]
pub enum CGOPointerButton {
    PointerButtonNone,
    PointerButtonLeft,
    PointerButtonRight,
    PointerButtonMiddle,
}

impl From<CGOPointer> for PointerEvent {
    fn from(p: CGOPointer) -> PointerEvent {
        PointerEvent {
            x: p.x,
            y: p.y,
            button: match p.button {
                CGOPointerButton::PointerButtonNone => PointerButton::None,
                CGOPointerButton::PointerButtonLeft => PointerButton::Left,
                CGOPointerButton::PointerButtonRight => PointerButton::Right,
                CGOPointerButton::PointerButtonMiddle => PointerButton::Middle,
            },
            down: p.down,
        }
    }
}

#[no_mangle]
pub extern "C" fn write_rdp_pointer(client_ptr: *mut Client, pointer: CGOPointer) -> CGOError {
    let client = unsafe { Client::from_ptr(client_ptr) };
    let client = match client {
        Some(client) => client,
        None => {
            return to_cgo_error("invalid Rust client pointer".to_string());
        }
    };
    let res = client
        .rdp_client
        .lock()
        .unwrap()
        .write(RdpEvent::Pointer(pointer.into()));
    if let Err(e) = res {
        to_cgo_error(format!("failed writing RDP pointer event: {:?}", e))
    } else {
        CGO_OK
    }
}

// A CGO-compatible copy of KeyboardEvent.
#[repr(C)]
#[derive(Copy, Clone)]
pub struct CGOKey {
    pub code: u16,
    pub down: bool,
}

impl From<CGOKey> for KeyboardEvent {
    fn from(k: CGOKey) -> KeyboardEvent {
        KeyboardEvent {
            code: k.code,
            down: k.down,
        }
    }
}

#[no_mangle]
pub extern "C" fn write_rdp_keyboard(client_ptr: *mut Client, key: CGOKey) -> CGOError {
    let client = unsafe { Client::from_ptr(client_ptr) };
    let client = match client {
        Some(client) => client,
        None => {
            return to_cgo_error("invalid Rust client pointer".to_string());
        }
    };
    let res = client
        .rdp_client
        .lock()
        .unwrap()
        .write(RdpEvent::Key(key.into()));
    if let Err(e) = res {
        to_cgo_error(format!("failed writing RDP keyboard event: {:?}", e))
    } else {
        CGO_OK
    }
}

#[no_mangle]
pub extern "C" fn close_rdp(client_ptr: *mut Client) -> CGOError {
    let client = unsafe { Client::from_ptr(client_ptr) };
    let client = match client {
        Some(client) => client,
        None => {
            return to_cgo_error("invalid Rust client pointer".to_string());
        }
    };
    if let Err(e) = client.rdp_client.lock().unwrap().shutdown() {
        to_cgo_error(format!("failed writing RDP keyboard event: {:?}", e))
    } else {
        CGO_OK
    }
}

#[no_mangle]
pub extern "C" fn free_rdp(client_ptr: *mut Client) {
    unsafe { drop(Client::from_raw(client_ptr)) }
}

#[no_mangle]
pub unsafe extern "C" fn free_rust_string(s: *mut c_char) {
    let _ = CString::from_raw(s);
}

fn from_go_string(s: *mut c_char) -> String {
    unsafe { CStr::from_ptr(s).to_string_lossy().into_owned().clone() }
}

// CGOError is an alias for a C string pointer, for C API clarity.
pub type CGOError = *mut c_char;

const CGO_OK: CGOError = ptr::null_mut();

fn to_cgo_error(s: String) -> CGOError {
    CString::new(s).expect("CString::new failed").into_raw()
}

// from_cgo_error copies CGOError into a String and frees the underlying Go memory.
fn from_cgo_error(e: CGOError) -> String {
    let s = from_go_string(e);
    unsafe {
        free_go_string(e);
    }
    s
}

extern "C" {
    fn free_go_string(s: *mut c_char);
    fn handle_bitmap(client_ref: i64, b: CGOBitmap) -> CGOError;
}

pub(crate) type Payload = Cursor<Vec<u8>>;
