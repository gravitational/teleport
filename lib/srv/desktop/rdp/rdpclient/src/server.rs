use anyhow::Result;
use ironrdp_connector::DesktopSize;
use ironrdp_pdu::rdp::capability_sets::{client_codecs_capabilities, BitmapCodecs};
use ironrdp_server::{
    KeyboardEvent, MouseEvent, RdpServer, RdpServerDisplay, RdpServerDisplayUpdates,
    RdpServerInputHandler, ServerEvent,
};

// struct X11InputHandler {}
//
// impl RdpServerInputHandler for X11InputHandler {
//     fn keyboard(&mut self, event: KeyboardEvent) {
//         todo!()
//     }
//
//     fn mouse(&mut self, event: MouseEvent) {
//         todo!()
//     }
// }
//
// struct X11DisplayHandler {
//     size: DesktopSize,
// }
//
// impl RdpServerDisplay for X11DisplayHandler {
//     async fn size(&mut self) -> DesktopSize {
//         todo!()
//     }
//
//     async fn updates(&mut self) -> Result<Box<dyn RdpServerDisplayUpdates>> {
//         todo!()
//     }
// }
//
// async fn start_server() -> Result<()> {
//     let codecs = client_codecs_capabilities(&[]).unwrap();
//     let mut rdp_server = RdpServer::builder()
//         .with_addr(([127, 0, 0, 1], 0))
//         .with_no_security()
//         .with_input_handler(X11InputHandler {})
//         .with_display_handler(X11DisplayHandler {
//             size: DesktopSize {
//                 width: 500,
//                 height: 500,
//             },
//         })
//         .with_bitmap_codecs(codecs)
//         .build();
//     tokio::spawn(async move {
//         rdp_server.run().await;
//     });
//     Ok(())
// }
