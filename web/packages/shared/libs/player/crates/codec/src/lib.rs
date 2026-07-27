pub mod error;
pub mod incoming;
pub mod messages;
pub mod tdp;
pub mod tdpb;

use crate::error::CodecError;
use crate::incoming::InboundMessage;
use crate::messages::*;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum Mode {
    Tdp,
    Tdpb,
}

pub struct Codec {
    mode: Mode,
}

impl Default for Codec {
    fn default() -> Self {
        Self::new()
    }
}

/// Generates `Codec::$name(&self, m: &$ty) -> Result<Vec<u8>, _>` that
/// dispatches to the active wire mode. Both `tdp::encode` and `tdpb::encode`
/// must export a function with the matching signature for each entry.
macro_rules! delegate_encode {
    ($( $name:ident ( $ty:ty ) ),* $(,)?) => {
        $(
            pub fn $name(&self, m: &$ty) -> Result<Vec<u8>, CodecError> {
                match self.mode {
                    Mode::Tdp  => crate::tdp::encode::$name(m),
                    Mode::Tdpb => crate::tdpb::encode::$name(m),
                }
            }
        )*
    };
}

impl Codec {
    /// Starts in legacy TDP. A current-proxy cluster sends a `TdpbUpgrade` marker as the first inbound frame; the driver must observe it and call [`Codec::upgrade_to_tdpb`] before sending anything else.
    #[must_use]
    pub fn new() -> Self {
        Self { mode: Mode::Tdp }
    }

    #[must_use]
    pub fn is_tdpb(&self) -> bool {
        self.mode == Mode::Tdpb
    }

    pub fn decode(&self, bytes: bytes::Bytes) -> Result<InboundMessage, CodecError> {
        match self.mode {
            Mode::Tdp => tdp::decode::decode(bytes),
            Mode::Tdpb => tdpb::decode::decode(bytes),
        }
    }

    /// Caller must follow with [`Codec::client_hello`]; TDPB has no implicit re-handshake.
    pub fn upgrade_to_tdpb(&mut self) {
        debug_assert!(self.mode == Mode::Tdp, "double upgrade");
        self.mode = Mode::Tdpb;
    }

    delegate_encode! {
        client_hello(ClientHello),
        session_selection(SessionSelection),
        screen_spec(ScreenSpec),
        sync_keys(SyncKeys),
        mouse_move(MouseMove),
        mouse_button(MouseButton),
        mouse_wheel(MouseWheel),
        keyboard_button(KeyboardButton),
        rdp_response(RdpResponsePdu),
        refresh_rect(RefreshRect),
        clipboard(ClipboardIn),
        mfa_response(MfaResponse),
        ping(Ping),
        share_dir_announce(ShareDirAnnounce),
        share_dir_remove(ShareDirRemove),
        share_dir_response(ShareDirResponse),
    }
}
