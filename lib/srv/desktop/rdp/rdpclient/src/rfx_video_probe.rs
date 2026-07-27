// Stub DVC handler for the RemoteFX video-streaming channels Windows offers
// in multi-monitor mode (`Microsoft::Windows::RDS::Video::Control::v08.01`,
// `Microsoft::Windows::RDS::Video::Data::v08.01`,
// `Microsoft::Windows::RDS::Geometry::v08.01`). Without a handler IronRDP
// rejects the channel and we can't tell whether Windows would have sent
// secondary-monitor pixels over it. This probe accepts the channel and logs
// every payload's size + first-byte hex so we can answer that question
// before deciding whether to invest in a full RFX-PA / video decoder.
//
// Not a real implementation — it discards all data.

use ironrdp_core::impl_as_any;
use ironrdp_dvc::{DvcMessage, DvcProcessor};
use ironrdp_pdu::PduResult;
use log::info;

pub struct ProbeDvcProcessor {
    name: &'static str,
    payload_count: u64,
    payload_bytes_total: u64,
}

impl_as_any!(ProbeDvcProcessor);

impl ProbeDvcProcessor {
    pub fn new(name: &'static str) -> Self {
        Self {
            name,
            payload_count: 0,
            payload_bytes_total: 0,
        }
    }
}

impl DvcProcessor for ProbeDvcProcessor {
    fn channel_name(&self) -> &str {
        self.name
    }

    fn start(&mut self, channel_id: u32) -> PduResult<Vec<DvcMessage>> {
        info!(
            "[multimon-marker][rfx-probe] channel started: {} id={}",
            self.name, channel_id
        );
        Ok(Vec::new())
    }

    fn process(&mut self, channel_id: u32, payload: &[u8]) -> PduResult<Vec<DvcMessage>> {
        self.payload_count += 1;
        self.payload_bytes_total += payload.len() as u64;
        let head: Vec<String> = payload
            .iter()
            .take(16)
            .map(|b| format!("{:02x}", b))
            .collect();
        info!(
            "[multimon-marker][rfx-probe] {} id={} payload#{} bytes={} total={} head=[{}]",
            self.name,
            channel_id,
            self.payload_count,
            payload.len(),
            self.payload_bytes_total,
            head.join(" "),
        );
        Ok(Vec::new())
    }
}
