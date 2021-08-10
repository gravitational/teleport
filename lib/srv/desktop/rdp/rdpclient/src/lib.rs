#[macro_use]
extern crate lazy_static;

use libc::{fd_set, select, FD_SET};
use rdp::core::client::{Connector, RdpClient};
use rdp::core::event::*;
use rdp::model::error::*;
use std::collections::HashMap;
use std::convert::TryFrom;
use std::ffi::{CStr, CString};
use std::mem;
use std::net::TcpStream;
use std::os::raw::c_char;
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

// connect_rdp establishes an RDP connection to go_addr with the provided credentials and screen
// size. If succeeded, the client is internally registered under client_ref. When done with the
// connection, the caller must call close_rdp.
#[no_mangle]
pub extern "C" fn connect_rdp(
    go_addr: *const c_char,
    go_username: *const c_char,
    go_password: *const c_char,
    screen_width: u16,
    screen_height: u16,
    client_ref: i64,
) -> *mut c_char {
    // Convert from C to Rust types.
    let addr = from_go_string(go_addr);
    let username = from_go_string(go_username);
    let password = from_go_string(go_password);

    // Connect and authenticate.
    let tcp = match TcpStream::connect(&addr) {
        Ok(tcp) => tcp,
        Err(e) => {
            return CString::new(format!("failed TCP connection to {}: {:?}", &addr, e))
                .expect("CString::new failed")
                .into_raw()
        }
    };
    let tcp_fd = tcp.as_raw_fd() as usize;
    let mut connector = Connector::new()
        .screen(screen_width, screen_height)
        .credentials(".".to_string(), username.to_string(), password.to_string());
    let client = match connector.connect(tcp) {
        Ok(client) => client,
        Err(e) => {
            return CString::new(format!("failed RDP connection to {}: {:?}", &addr, e))
                .expect("CString::new failed")
                .into_raw()
        }
    };

    register_client(
        client_ref,
        Client {
            rdp_client: client,
            tcp_fd: tcp_fd,
        },
    );
    std::ptr::null_mut()
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
    type Error = Error;

    fn try_from(e: BitmapEvent) -> Result<Self, Self::Error> {
        let mut res = CGOBitmap {
            dest_left: e.dest_left,
            dest_top: e.dest_top,
            dest_right: e.dest_right,
            dest_bottom: e.dest_bottom,
            data_ptr: std::ptr::null_mut(),
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
pub extern "C" fn read_rdp_output(client_ref: i64) -> *mut c_char {
    let mut tcp_fd = 0;
    if let Err(e) = with_client(&client_ref, |client| -> Result<(), ClientError> {
        tcp_fd = client.lock().unwrap().tcp_fd;
        Ok(())
    }) {
        return CString::new(format!(
            "failed looking up TCP file descriptor for client {}: {:?}",
            client_ref, e
        ))
        .expect("CString::new failed")
        .into_raw();
    }
    // Read incoming events.
    // TODO: this doesn't always unblock after client.shutdown() was called. Figure out why.
    while wait_for_fd(tcp_fd as usize) {
        if let Err(e) = with_client(&client_ref, |client| -> Result<(), ClientError> {
            let mut err: *mut c_char = std::ptr::null_mut();
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
                        err = handle_bitmap(client_ref, cbitmap) as *mut c_char;
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
            if err != std::ptr::null_mut() {
                let err_str = from_go_string(err);
                unsafe {
                    free_go_string(err);
                }
                Err(ClientError::Other(format!(
                    "failed forwarding RDP bitmap frame: {}",
                    err_str
                )))
            } else {
                Ok(())
            }
        }) {
            return CString::new(format!("failed reading RDP bitmap frame: {:?}", e))
                .expect("CString::new failed")
                .into_raw();
        }
    }
    std::ptr::null_mut()
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
pub extern "C" fn write_rdp_pointer(client_ref: i64, pointer: CGOPointer) -> *mut c_char {
    if let Err(e) = with_client(&client_ref, |client| -> Result<(), ClientError> {
        Ok(client
            .lock()
            .unwrap()
            .rdp_client
            .write(RdpEvent::Pointer(pointer.into()))?)
    }) {
        CString::new(format!("failed writing RDP pointer event: {:?}", e))
            .expect("CString::new failed")
            .into_raw()
    } else {
        std::ptr::null_mut()
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
pub extern "C" fn write_rdp_keyboard(client_ref: i64, key: CGOKey) -> *mut c_char {
    if let Err(e) = with_client(&client_ref, |client| -> Result<(), ClientError> {
        Ok(client
            .lock()
            .unwrap()
            .rdp_client
            .write(RdpEvent::Key(key.into()))?)
    }) {
        CString::new(format!("failed writing RDP keyboard event: {:?}", e))
            .expect("CString::new failed")
            .into_raw()
    } else {
        std::ptr::null_mut()
    }
}

#[no_mangle]
pub extern "C" fn close_rdp(client_ref: i64) -> *mut c_char {
    if let Err(e) = with_client(&client_ref, |client| -> Result<(), ClientError> {
        Ok(client.lock().unwrap().rdp_client.shutdown()?)
    }) {
        return CString::new(format!("failed writing RDP keyboard event: {:?}", e))
            .expect("CString::new failed")
            .into_raw();
    }
    unregister_client(&client_ref);
    std::ptr::null_mut()
}

#[no_mangle]
pub unsafe extern "C" fn free_rust_string(s: *mut c_char) {
    let _ = CString::from_raw(s);
}

fn from_go_string(s: *const c_char) -> &'static str {
    unsafe {
        CStr::from_ptr(s)
            .to_str()
            .expect("got a non-UTF8 string from Go")
    }
}

extern "C" {
    fn free_go_string(s: *mut c_char);
    fn handle_bitmap(client_ref: i64, b: CGOBitmap) -> *mut c_char;
}
