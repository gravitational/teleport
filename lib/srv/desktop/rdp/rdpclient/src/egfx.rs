

use ironrdp_core::impl_as_any;
use ironrdp_dvc::{DvcClientProcessor, DvcMessage, DvcProcessor};
use ironrdp_pdu::{PduError, PduResult};

pub const EGFX_CHANNEL_NAME: &str = ironrdp_egfx::CHANNEL_NAME;

pub trait DvcHandler: Send {
    fn start(&mut self, channel_id: u32, channel_name: String) -> Result<(), PduError>;
    fn process(&mut self, channel_id: u32, payload: &[u8]) -> Result<(), PduError>;
    fn stop(&mut self, _channel_id: u32) {}
}

pub struct PassthroughDVC{
    name: &'static str,
    handler: Box<dyn DvcHandler>,
}

impl PassthroughDVC{
    pub fn new(name: &'static str, handler: Box<dyn DvcHandler>) -> Self {
        Self{
            name: name,
            handler: handler,
        }
    }
}

impl_as_any!(PassthroughDVC);

impl DvcProcessor for PassthroughDVC {
     /// The name of the channel, e.g. "Microsoft::Windows::RDS::DisplayControl"
    fn channel_name(&self) -> &str {
        self.name
    }

    /// Returns any messages that should be sent immediately
    /// upon the channel being created.
    fn start(&mut self, channel_id: u32) -> PduResult<Vec<DvcMessage>> {
        self.handler.start(channel_id, self.channel_name().to_string())?;
        Ok(vec![])
    }

    fn process(&mut self, channel_id: u32, payload: &[u8]) -> PduResult<Vec<DvcMessage>> {
        self.handler.process(channel_id, payload)?;
        Ok(vec![])
    }

    fn close(&mut self, _channel_id: u32) {}
}

impl DvcClientProcessor for PassthroughDVC {}