use libc::{fd_set, select, FD_SET};
use rdp::core::client::{Connector, RdpClient};
use rdp::core::event::*;
use rdp::model::error::Error as RdpError;
use std::convert::TryFrom;
use std::ffi::{CStr, CString};
use std::io::Error as IoError;
use std::mem;
use std::net::TcpStream;
use std::os::raw::c_char;
use std::os::unix::io::AsRawFd;
use std::ptr;
use std::sync::{Arc, Mutex};

// Client has an unusual lifetime:
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
    fn into_raw(self) -> *mut Client {
        Box::into_raw(Box::new(self))
    }
    unsafe fn from_raw<'a>(ptr: *mut Client) -> &'a mut Client {
        ptr.as_mut().unwrap()
    }
    unsafe fn free(&mut self) {
        Box::from_raw(self as *mut Client);
    }
}

#[repr(C)]
pub struct ClientOrError {
    client: *mut Client,
    err: *mut c_char,
}

impl From<Result<Client, ConnectError>> for ClientOrError {
    fn from(r: Result<Client, ConnectError>) -> ClientOrError {
        match r {
            Ok(client) => ClientOrError {
                client: client.into_raw(),
                err: std::ptr::null_mut(),
            },
            Err(e) => ClientOrError {
                client: std::ptr::null_mut(),
                err: CString::new(format!("{:?}", e))
                    .expect("CString::new failed")
                    .into_raw(),
            },
        }
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
) -> ClientOrError {
    // Convert from C to Rust types.
    let addr = from_go_string(go_addr);
    let username = from_go_string(go_username);
    let password = from_go_string(go_password);

    connect_rdp_inner(addr, username, password, screen_width, screen_height).into()
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
    username: &str,
    password: &str,
    screen_width: u16,
    screen_height: u16,
) -> Result<Client, ConnectError> {
    // Connect and authenticate.
    let tcp = TcpStream::connect(addr)?;
    let tcp_fd = tcp.as_raw_fd() as usize;
    let mut connector = Connector::new()
        .screen(screen_width, screen_height)
        .credentials(".".to_string(), username.to_string(), password.to_string());
    let client = connector.connect(tcp)?;

    Ok(Client {
        rdp_client: Arc::new(Mutex::new(client)),
        tcp_fd: tcp_fd,
    })
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
pub extern "C" fn read_rdp_output(client_ptr: *mut Client, client_ref: i64) -> *mut c_char {
    let client = unsafe { Client::from_raw(client_ptr) };
    let tcp_fd = client.tcp_fd;
    // Read incoming events.
    // TODO: this doesn't always unblock after client.shutdown() was called. Figure out why.
    while wait_for_fd(tcp_fd as usize) {
        let mut err: *mut c_char = std::ptr::null_mut();
        let res = client
            .rdp_client
            .lock()
            .unwrap()
            .read(|rdp_event| match rdp_event {
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
            });
        if let Err(e) = res {
            return CString::new(format!("failed forwarding RDP bitmap frame: {:?}", e))
                .expect("CString::new failed")
                .into_raw();
        };
        if err != std::ptr::null_mut() {
            let err_str = from_go_string(err);
            unsafe {
                free_go_string(err);
            }
            return CString::new(format!("failed forwarding RDP bitmap frame: {}", err_str))
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
pub extern "C" fn write_rdp_pointer(client_ptr: *mut Client, pointer: CGOPointer) -> *mut c_char {
    let client = unsafe { Client::from_raw(client_ptr) };
    let res = client
        .rdp_client
        .lock()
        .unwrap()
        .write(RdpEvent::Pointer(pointer.into()));
    if let Err(e) = res {
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
pub extern "C" fn write_rdp_keyboard(client_ptr: *mut Client, key: CGOKey) -> *mut c_char {
    let client = unsafe { Client::from_raw(client_ptr) };
    let res = client
        .rdp_client
        .lock()
        .unwrap()
        .write(RdpEvent::Key(key.into()));
    if let Err(e) = res {
        CString::new(format!("failed writing RDP keyboard event: {:?}", e))
            .expect("CString::new failed")
            .into_raw()
    } else {
        std::ptr::null_mut()
    }
}

#[no_mangle]
pub extern "C" fn close_rdp(client_ptr: *mut Client) -> *mut c_char {
    let client = unsafe { Client::from_raw(client_ptr) };
    let res = client.rdp_client.lock().unwrap().shutdown();
    unsafe { client.free() };
    if let Err(e) = res {
        CString::new(format!("failed writing RDP keyboard event: {:?}", e))
            .expect("CString::new failed")
            .into_raw()
    } else {
        std::ptr::null_mut()
    }
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
