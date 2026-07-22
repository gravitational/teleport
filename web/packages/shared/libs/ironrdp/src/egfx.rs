

use std::collections::HashMap;

use std::hash::Hash;
use std::sync::mpsc::{Sender};
use ironrdp_dvc::DvcMessage;
use ironrdp_egfx::client::{GraphicsPipelineClient, GraphicsPipelineHandler};
use ironrdp_egfx::pdu::{PixelFormat, Codec2Type};
use ironrdp_pdu::{PduError, geometry::InclusiveRectangle};
use tracing::warn;

// Need to use GraphicsPipelineClient
// - Must provide a handler
// - Handler needs a way to queue up "draw" operations


struct TeleportEGFXProcessor(GraphicsPipelineClient);


pub struct ImageData {
    pub location: InclusiveRectangle,
    pub data: Vec<u8>
}

type EgfxResult = Result<Vec<ImageData>, PduError>;

type CacheKey = u64;
type CacheSlot = u16;

struct CacheEntry {
    key: CacheKey,

}

struct Surface {
    codec_id: Codec2Type,
    codec_context_id: u32,
    pixel_format: PixelFormat,
    bitmap_data: Vec<u8>,
}

struct EncodingContext (());

pub struct TeleportEgfxHandler {
    image_data_sender: Sender<Vec<ImageData>>,
    pdu_sender: Sender<Vec<DvcMessage>>,
    bitmap_cache: HashMap<CacheSlot, CacheEntry>,
    surfaces: HashMap<u16, Surface>,
    encoding_contexts: HashMap<u32, EncodingContext>
}

impl TeleportEgfxHandler {
    pub fn new(image_data_sender: Sender<Vec<ImageData>>, pdu_sender: Sender<Vec<DvcMessage>>) -> Self {
        Self{
            image_data_sender,
            pdu_sender,
            bitmap_cache: HashMap::new(),
            surfaces: HashMap::new(),
            encoding_contexts: HashMap::new(),
        }
    }
}

impl GraphicsPipelineHandler for TeleportEgfxHandler {

    /****** WRITE TO SCREEN ******/
    // Primary method for mapping a surface to the output buffer (the actual screen)
    fn on_surface_mapped(&mut self, _surface_id: u16, _origin_x: u32, _origin_y: u32) {
        
    }

    // Another method for mapping a surface to the output buffer (the actual screen)
    // where the surface must be scaled to fit in target rectangle
    fn on_map_surface_to_scaled_output(&mut self, _pdu: &ironrdp_egfx::pdu::MapSurfaceToScaledOutputPdu) {
        
    }

    // Hopefully only called by clients that open a RAIL window with MS_RDPERP.
    fn on_map_surface_to_scaled_window(&mut self, _pdu: &ironrdp_egfx::pdu::MapSurfaceToScaledWindowPdu) {
         warn!("unexpected mapping to scaled RAIL window") 
    }

        // Hopefully only called by clients that open a RAIL window with MS_RDPERP.
    fn on_map_surface_to_window(&mut self, _pdu: &ironrdp_egfx::pdu::MapSurfaceToWindowPdu) {
        warn!("unexpected mapping to RAIL window")    
    }

    /****** SURFACE MANAGEMENT ******/
    fn on_surface_created(&mut self, _surface: &ironrdp_egfx::client::Surface) {
        
    }

     // Delete the surface
    fn on_surface_deleted(&mut self, _surface_id: u16) {
        
    }

    // on_wire_to_surface2 and on_bitmap_updated both write data to a surface
    // on_wire_to_surface2 is called for progressive codecs (RFX). This data is still encoded    
    fn on_wire_to_surface2(&mut self, pdu: &ironrdp_egfx::pdu::WireToSurface2Pdu) {
        
    }
    // on_bitmap_updated is called when the GraphicsClient was able to decode the data
    // (the bitmap data is already decoded). In an ideal world, this is the only handler
    // that the client invokes when writing to a surface, but for it only supports Avc420.
    fn on_bitmap_updated(&mut self, _update: &ironrdp_egfx::client::BitmapUpdate) {
        
    }

    fn on_surface_to_surface(&mut self, _pdu: &ironrdp_egfx::pdu::SurfaceToSurfacePdu) {
        
    }

    // Possibly a wiretosurface for an unsupported codec
    fn on_unhandled_pdu(&mut self, pdu: &ironrdp_egfx::pdu::GfxPdu) {
        // The graphics client will either pass us an unknown pdu type
        // or a wiretosurface1 with an unknown codec
        if !matches!(pdu, ironrdp_egfx::pdu::GfxPdu::WireToSurface1(_)) {
            warn!("received unknown pdu type")
        }
        // Ideally we can match the provided codec, decode, and write to the specified surface,
    }

    fn on_solid_fill(&mut self, _pdu: &ironrdp_egfx::pdu::SolidFillPdu) {
        
    }

    fn on_delete_encoding_context(&mut self, _pdu: &ironrdp_egfx::pdu::DeleteEncodingContextPdu) {
        
    }

    /****** FRAME MANAGEMENT ******/
    fn on_frame_complete(&mut self, _frame_id: u32) {
        
    }

    /****** CACHE OPERATIONS ******/
    fn on_cache_to_surface(&mut self, _pdu: &ironrdp_egfx::pdu::CacheToSurfacePdu) {
        
    }

    fn on_surface_to_cache(&mut self, _pdu: &ironrdp_egfx::pdu::SurfaceToCachePdu) {
        
    }


    fn on_evict_cache_entry(&mut self, _pdu: &ironrdp_egfx::pdu::EvictCacheEntryPdu) {
        
    }
    // Not implementing import offer
   


}



//impl TeleportEGFXProcessor {
//
//    pub fn new(sender: Sender<ImageData>) -> Self {
//        
//    }
//
//    pub fn start(&mut self, channel_id: u32, channel_name: String) -> EgfxResult {
//        Ok(vec![])
//    }
//
//    pub fn process(&mut self, channel_id: u32, payload: &[u32]) -> EgfxResult {
//        Ok(vec![])
//    }
//
//}
