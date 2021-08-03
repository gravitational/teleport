#[macro_use]
extern crate lazy_static;

use libc::{fd_set, select, FD_SET};
use rdp::core::client::{Connector, RdpClient};
use rdp::core::event::*;
use rdp::model::error::*;
use std::collections::HashMap;
use std::convert::TryFrom;
use std::mem;
use std::net::TcpStream;
use std::os::unix::io::AsRawFd;
use std::ptr;
use std::sync::{Arc, Mutex};

struct Client {
    rdp_client: RdpClient<TcpStream>,
    tcp_fd: usize,
}
type SyncRdpClient = Arc<Mutex<Client>>;

// Rust-side registry of clients, to allow Go to reference a specific client in calls after
// connect_rdp.
lazy_static! {
    static ref RDP_CLIENTS: Arc<Mutex<HashMap<i64, SyncRdpClient>>> =
        Arc::new(Mutex::new(HashMap::new()));
}

fn register_client(client_ref: i64, client: Client) {
    RDP_CLIENTS
        .lock()
        .unwrap()
        .insert(client_ref, Arc::new(Mutex::new(client)));
}

fn unregister_client(client_ref: &i64) {
    RDP_CLIENTS.lock().unwrap().remove(client_ref);
}

#[derive(Debug)]
enum ClientError {
    RDP(Error),
    ClientNotFound,
    Other(String),
}

impl From<Error> for ClientError {
    fn from(e: Error) -> ClientError {
        ClientError::RDP(e)
    }
}

fn with_client<F: FnMut(&SyncRdpClient) -> Result<(), ClientError>>(
    client_ref: &i64,
    mut f: F,
) -> Result<(), ClientError> {
    match RDP_CLIENTS.lock().unwrap().get(client_ref) {
        Some(client) => f(client),
        None => Err(ClientError::ClientNotFound),
    }
}

// CGOString is a CGO-compatible definition of a UTF-8 string that can be used in Go and Rust.
// This string is not null-terminated.
#[repr(C)]
pub struct CGOString {
    // Memory is freed on the receiving side.
    data: *mut u8,
    len: u16,
}

// CGOError is just a CGOString used when returning error messages.
// A "null" CGOError means no error, like in Go.
type CGOError = CGOString;

impl CGOString {
    fn null() -> CGOString {
        CGOString {
            data: std::ptr::null_mut(),
            len: 0,
        }
    }
    fn is_null(&self) -> bool {
        self.data == std::ptr::null_mut()
    }
}

impl From<CGOString> for String {
    fn from(s: CGOString) -> String {
        unsafe { String::from_raw_parts(s.data, s.len.into(), s.len.into()) }
    }
}

// Important: when converting String to CGOString, the caller is responsible for manually freeing
// the memory at CGOString.data.
impl From<String> for CGOString {
    fn from(mut s: String) -> CGOString {
        let cs = CGOString {
            data: s.as_mut_ptr(),
            len: s.len() as u16,
        };
        // Tell Rust to forget about the underlying memory, so that the caller can free it later.
        std::mem::forget(s);
        cs
    }
}

// connect_rdp establishes an RDP connection to go_addr with the provided credentials and screen
// size. If succeeded, the client is internally registered under client_ref. When done with the
// connection, the caller must call close_rdp.
#[no_mangle]
pub extern "C" fn connect_rdp(
    go_addr: CGOString,
    go_username: CGOString,
    go_password: CGOString,
    screen_width: u16,
    screen_height: u16,
    client_ref: i64,
) -> CGOError {
    // Convert from C to Rust types.
    let addr = String::from(go_addr);
    let username = String::from(go_username);
    let password = String::from(go_password);

    // Connect and authenticate.
    let tcp = match TcpStream::connect(&addr) {
        Ok(tcp) => tcp,
        Err(e) => return format!("failed TCP connection to {}: {:?}", &addr, e).into(),
    };
    let tcp_fd = tcp.as_raw_fd() as usize;
    let mut connector = Connector::new()
        .screen(screen_width, screen_height)
        .credentials(".".to_string(), username.to_string(), password.to_string());
    let client = match connector.connect(tcp) {
        Ok(client) => client,
        Err(e) => return format!("failed RDP connection to {}: {:?}", &addr, e).into(),
    };

    register_client(
        client_ref,
        Client {
            rdp_client: client,
            tcp_fd: tcp_fd,
        },
    );
    CGOError::null()
}

#[repr(C)]
pub struct CGOBitmap {
    pub dest_left: u16,
    pub dest_top: u16,
    pub dest_right: u16,
    pub dest_bottom: u16,
    // Memory is freed on the Rust side.
    pub data_ptr: *const u8,
    pub data_len: usize,
}

impl TryFrom<BitmapEvent> for CGOBitmap {
    type Error = Error;

    fn try_from(e: BitmapEvent) -> Result<Self, Self::Error> {
        let mut res = CGOBitmap {
            dest_left: e.dest_left,
            dest_top: e.dest_top,
            dest_right: e.dest_right,
            dest_bottom: e.dest_bottom,
            data_ptr: std::ptr::null(),
            data_len: 0,
        };

        // e.decompress consumes e, so we need to call it separately, after populating the fields
        // above.
        let data = if e.is_compress {
            e.decompress()?
        } else {
            e.data
        };
        res.data_ptr = data.as_ptr();
        res.data_len = data.len();

        Ok(res)
    }
}

// TODO: this is Linux-specific, also implement for Windows.
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
pub extern "C" fn read_rdp_output(
    client_ref: i64,
    handle_bitmap: unsafe extern "C" fn(i64, CGOBitmap) -> CGOError,
) -> CGOError {
    let mut tcp_fd = 0;
    if let Err(e) = with_client(&client_ref, |client| -> Result<(), ClientError> {
        tcp_fd = client.lock().unwrap().tcp_fd;
        Ok(())
    }) {
        return format!(
            "failed looking up TCP file descriptor for client {}: {:?}",
            client_ref, e
        )
        .into();
    }
    // Read incoming events.
    // TODO: this doesn't always unblock after client.shutdown() was called. Figure out why.
    while wait_for_fd(tcp_fd as usize) {
        if let Err(e) = with_client(&client_ref, |client| -> Result<(), ClientError> {
            let mut err = CGOError::null();
            let mut client = client.lock().unwrap();
            client.rdp_client.read(|rdp_event| match rdp_event {
                RdpEvent::Bitmap(bitmap) => {
                    let cbitmap = match CGOBitmap::try_from(bitmap) {
                        Ok(cb) => cb,
                        Err(e) => {
                            println!(
                                "failed to convert RDP bitmap to CGO representation: {:?}",
                                e
                            );
                            return;
                        }
                    };
                    unsafe {
                        err = handle_bitmap(client_ref, cbitmap);
                    };
                }
                // These should never really be sent by the server to us.
                RdpEvent::Pointer(_) => {
                    println!("got unexpected pointer event from RDP server, ignoring");
                }
                RdpEvent::Key(_) => {
                    println!("got unexpected keyboard event from RDP server, ignoring");
                }
            })?;
            if !err.is_null() {
                Err(ClientError::Other(format!(
                    "failed forwarding RDP bitmap frame: {:?}",
                    String::from(err)
                )))
            } else {
                Ok(())
            }
        }) {
            return format!("failed reading RDP bitmap frame: {:?}", e).into();
        }
    }
    CGOError::null()
}

// A CGO-compatible copy of PointerEvent.
#[repr(C)]
#[derive(Copy, Clone)]
pub struct Pointer {
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

impl From<Pointer> for PointerEvent {
    fn from(p: Pointer) -> PointerEvent {
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
pub extern "C" fn write_rdp_pointer(client_ref: i64, pointer: Pointer) -> CGOError {
    if let Err(e) = with_client(&client_ref, |client| -> Result<(), ClientError> {
        Ok(client
            .lock()
            .unwrap()
            .rdp_client
            .write(RdpEvent::Pointer(pointer.into()))?)
    }) {
        format!("failed writing RDP pointer event: {:?}", e).into()
    } else {
        CGOError::null()
    }
}

// A CGO-compatible copy of KeyboardEvent.
#[repr(C)]
#[derive(Copy, Clone)]
pub struct Key {
    pub code: u16,
    pub down: bool,
}

impl From<Key> for KeyboardEvent {
    fn from(k: Key) -> KeyboardEvent {
        KeyboardEvent {
            code: k.code,
            down: k.down,
        }
    }
}

#[no_mangle]
pub extern "C" fn write_rdp_keyboard(client_ref: i64, key: Key) -> CGOError {
    if let Err(e) = with_client(&client_ref, |client| -> Result<(), ClientError> {
        Ok(client
            .lock()
            .unwrap()
            .rdp_client
            .write(RdpEvent::Key(key.into()))?)
    }) {
        format!("failed writing RDP keyboard event: {:?}", e).into()
    } else {
        CGOError::null()
    }
}

#[no_mangle]
pub extern "C" fn close_rdp(client_ref: i64) -> CGOError {
    if let Err(e) = with_client(&client_ref, |client| -> Result<(), ClientError> {
        Ok(client.lock().unwrap().rdp_client.shutdown()?)
    }) {
        return format!("failed writing RDP keyboard event: {:?}", e).into();
    }
    unregister_client(&client_ref);
    CGOError::null()
}
