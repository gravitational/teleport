

use std::collections::{HashMap, HashSet};
use std::sync::mpsc::{Sender};
use ironrdp_core::{DecodeError, DecodeErrorKind, DecodeResult};
use ironrdp_dvc::DvcMessage;
use ironrdp_egfx::client::{GraphicsPipelineClient, GraphicsPipelineHandler};
use ironrdp_egfx::pdu::Codec1Type::{ClearCodec, Uncompressed};
use ironrdp_egfx::pdu::GfxPdu::WireToSurface1;
use ironrdp_egfx::pdu::{Codec1Type, Codec2Type, Color, PixelFormat};
use ironrdp_graphics::clearcodec::ClearCodecDecoder;
use ironrdp_pdu::geometry::ExclusiveRectangle;
use ironrdp_pdu::{PduError, geometry::InclusiveRectangle};
use ironrdp_graphics::progressive::{DecodedTile, ProgressiveDecodeError, ProgressiveDecoder};
use tracing::warn;

// Need to use GraphicsPipelineClient
// - Must provide a handler
// - Handler needs a way to queue up "draw" operations


// NOTES
// 1. Server groups related operations within a FRAME. We must acknowledge
//    each frame once all commands are complete. For MAP_TO_OUTPUT messages, this
//    means that the data must be rendered to the client's screen before acknowledging.
// 2. Frames are not interleaved, but we as the client can process multiple frames concurrently
//    ie, the server may send subsequent frames before the precedding frame(s) are ack'ed.
// 3. Surfaces are mapped to the visible canvas via "on_surface_mapped". Once a surface is
//    mapped, subsequent surface updates are automatically rendered *at the end of the frame*.
//    We must maintain a "Surface to Output Mapping" list.


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

#[derive(Clone)]
struct ScalingFactor {
    targetWidth: u32,
    targetHeight: u32,
}

#[derive(Clone)]
struct Mapping {
    origin_x: u32,
    origin_y: u32,
    scaling_factor: Option<ScalingFactor>
}

pub struct MappedSurface<'a, S: SurfaceEx> {
    pub surface: &'a S,
    pub mapping: Mapping,
}

struct Surface<S: SurfaceEx> {
    width: u16,
    height: u16,
    pixel_format: PixelFormat,
    surface: S,
    // Set when the surface is mapped.
    mapping: Option<Mapping>
}

impl <S: SurfaceEx> Surface<S> {
    fn apply_bitmap_update(&mut self, location: &ExclusiveRectangle, data: &Vec<u8>) {
        self.surface.update(location, data);
    }
}

trait SurfaceEx: Send {
    fn new() -> Self;
    fn update(&mut self, location: &ExclusiveRectangle, data: &Vec<u8>);
    fn copy(&self, dest: &mut Self, source_rect: &ExclusiveRectangle, points: Vec<(u16, u16)>);
    fn fill(&mut self, locations: &Vec<ExclusiveRectangle>, a: u8, r: u8, g: u8, b: u8);
}

enum EncodingContext {
    Uncompressed,
    CAVideo, /* RFX */
    ClearCodec,
}

fn decode_with_codec1(codec: Codec1Type, width: u16, height: u16, data: &[u8]) -> DecodeResult<Vec<u8>> {
    match codec {
        Uncompressed => Ok(Vec::from(data)),
        ClearCodec => ClearCodecDecoder::new().decode(data, width, height),        
        _ => Err(DecodeError::new("unknown/unsupported codec", DecodeErrorKind::UnsupportedValue { name: "codec", value: format!("{:?}", codec)}))
    }
}

trait ProgressiveCodec: Send {
    fn decode_bitmap(
        &mut self,
        codec_context_id: u32,
        surface_width: u16,
        surface_height: u16,
        bitmap_data: &[u8],
    ) -> Result<Vec<DecodedTile>, ProgressiveDecodeError>;
}

impl ProgressiveCodec for ProgressiveDecoder {
    fn decode_bitmap(
        &mut self,
        codec_context_id: u32,
        surface_width: u16,
        surface_height: u16,
        bitmap_data: &[u8],
    ) -> Result<Vec<DecodedTile>, ProgressiveDecodeError>
    {
        self.decode_bitmap(codec_context_id, surface_width, surface_height, bitmap_data)
    }
}

type DrawFn<S: SurfaceEx> = dyn FnMut(&Vec<MappedSurface<S>>) + Send;

pub struct TeleportEgfxHandler<S: SurfaceEx> {
    // Send channel for additional PDUs to be sent
    pdu_sender: Sender<Vec<DvcMessage>>,
    bitmap_cache: HashMap<CacheSlot, CacheEntry>,
    surfaces: HashMap<u16, Surface<S>>,
    encoding_contexts: HashMap<u32, Box<dyn ProgressiveCodec>>,
    // Callback that provides a list of surfaces to be rendered. Draw is synchronous.
    // The referenced surfaces will not be referenced after draw_cb returns, and we're free
    // to continue mutating each surface.
    draw_cb: Box<DrawFn<S>>,
    // track which surfaces are mapped
    dirty_surfaces: HashSet<u16>,
}

impl<S: SurfaceEx> TeleportEgfxHandler<S> {
    pub fn new(pdu_sender: Sender<Vec<DvcMessage>>, draw: Box<DrawFn<S>>) -> Self {
        Self{
            pdu_sender,
            bitmap_cache: HashMap::new(),
            surfaces: HashMap::new(),
            encoding_contexts: HashMap::new(),
            draw_cb: draw,
            dirty_surfaces: HashSet::new(),
        }
    }
}

impl <S: SurfaceEx> GraphicsPipelineHandler for TeleportEgfxHandler<S> {

    /****** WRITE TO SCREEN ******/
    // Primary method for mapping a surface to the output buffer (the actual screen)
    fn on_surface_mapped(&mut self, surface_id: u16, origin_x: u32, origin_y: u32) {
        let surface = self.surfaces.get_mut(&surface_id).expect("Missing surface");
        self.dirty_surfaces.insert(surface_id);
        surface.mapping = Some(Mapping{
            origin_x,
            origin_y,
            scaling_factor: None,
        })
    }

    // Another method for mapping a surface to the output buffer (the actual screen)
    // where the surface must be scaled to fit in target rectangle
    fn on_map_surface_to_scaled_output(&mut self, pdu: &ironrdp_egfx::pdu::MapSurfaceToScaledOutputPdu) {
        let surface = self.surfaces.get_mut(&pdu.surface_id).expect("Missing surface");
        self.dirty_surfaces.insert(pdu.surface_id);
        surface.mapping = Some(Mapping{
            origin_x: pdu.output_origin_x,
            origin_y: pdu.output_origin_y,
            scaling_factor: Some(ScalingFactor { targetWidth: pdu.target_width, targetHeight: pdu.target_height}),
        })        
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
    fn on_surface_created(&mut self, surface: &ironrdp_egfx::client::Surface) {
        let s = Surface{
            width: surface.width,
            height: surface.height,
            pixel_format: surface.pixel_format,
            surface: S::new(),
            mapping: None,
        };

        // TODO: Throw error if none
        _ = self.surfaces.insert(surface.id, s)
    }

     // Delete the surface
    fn on_surface_deleted(&mut self, surface_id: u16) {
        // TODO: Better error message
        self.surfaces.remove(&surface_id).expect("surface not found");
    }

    // on_wire_to_surface2 and on_bitmap_updated both write data to a surface
    // on_wire_to_surface2 is called for progressive codecs (RFX). We are responsible
    // for instantiation/recall of the encoding context and decoding the incoming bitmap.    
    fn on_wire_to_surface2(&mut self, pdu: &ironrdp_egfx::pdu::WireToSurface2Pdu) {
        if !matches!(pdu.codec_id, Codec2Type::RemoteFxProgressive) {
            warn!("only RemoteFX is supported for wire_to_surface2");
            return
        }

        if !self.encoding_contexts.contains_key(&pdu.codec_context_id) {
            self.encoding_contexts.insert(pdu.codec_context_id, Box::new(ProgressiveDecoder::new()));
        }

        let codec = self.encoding_contexts.get_mut(&pdu.codec_context_id).expect("unknown codec context");

        let surface = self.surfaces.get_mut(&pdu.surface_id).expect("missing surface");
        let res = codec.decode_bitmap(pdu.codec_context_id, surface.width, surface.height, &pdu.bitmap_data).unwrap();
        res.iter().for_each(|tile| {
            // Each tile is 64 x 64
            let location = ExclusiveRectangle{
                top: 64 * tile.y_idx,
                left: 64 * tile.x_idx,
                bottom: 64 * (tile.y_idx+1),
                right: 64 * (tile.x_idx+1),                
            };

            surface.apply_bitmap_update(&location, &tile.pixels);
        });
    }

    // on_bitmap_updated is called when the GraphicsClient was able to decode the data
    // (the bitmap data is already decoded). In an ideal world, this is the only handler
    // that the client invokes when writing to a surface, but for now it only supports Avc420.
    fn on_bitmap_updated(&mut self, update: &ironrdp_egfx::client::BitmapUpdate) {
        // BitmapUpdate.data is always RGBA8888 regardless of codec
        if let Some(surface) = self.surfaces.get_mut(&update.surface_id) {
            surface.apply_bitmap_update(&update.destination_rectangle, &update.data);
        } else {
            panic!("unknown surface")
        }
    }

    fn on_surface_to_surface(&mut self, pdu: &ironrdp_egfx::pdu::SurfaceToSurfacePdu) {
        // Defer to the "copy" function on SurfaceEx to handle this.
        // Hack?: is there a better way to grab two mutable refs to hashmap entries? Refcell maybe?
        let mut destination_surface = self.surfaces.remove(&pdu.destination_surface_id).expect("unknown destination surface");
        let source_surface = self.surfaces.get(&pdu.source_surface_id).expect("unknown source surface");
        
        // Convert points to a vec of tuples to avoid leaking EGFX internal types
        // through this module's public API.
        let pts: Vec<(u16, u16)> = pdu.destination_points.iter().map(|pt| (pt.x, pt.y)).collect();
        source_surface.surface.copy(&mut destination_surface.surface, &pdu.source_rectangle, pts);
        let _ = self.surfaces.insert(pdu.destination_surface_id, destination_surface);
    }

    // Possibly a wiretosurface for an unsupported codec
    fn on_unhandled_pdu(&mut self, pdu: &ironrdp_egfx::pdu::GfxPdu) {
        // The graphics client will either pass us an unknown pdu type
        // or a wiretosurface1 with an unknown codec
        if let Some(wire_to_surface_1) = match pdu {
            WireToSurface1(pdu) => Some(pdu),
            _ => None
        } {
            let surface = self.surfaces.get_mut(&wire_to_surface_1.surface_id).expect("unknown surface");
            let res = decode_with_codec1(wire_to_surface_1.codec_id, surface.width, surface.height, wire_to_surface_1.bitmap_data.as_slice());
            if let Ok(decoded_bitmap) = res {
                surface.apply_bitmap_update(&wire_to_surface_1.destination_rectangle, &decoded_bitmap);
            } else {
                panic!("Failed decode bitmap")
            }
        } else {
            panic!("Unknown pdu")
        }
    }

    fn on_solid_fill(&mut self, pdu: &ironrdp_egfx::pdu::SolidFillPdu) {
        let Color {b, g, r, xa} = pdu.fill_pixel;
        self.surfaces.get_mut(&pdu.surface_id).expect("missing surface").surface.fill(&pdu.rectangles, xa, r, g, b);
    }

    fn on_delete_encoding_context(&mut self, pdu: &ironrdp_egfx::pdu::DeleteEncodingContextPdu) {
        self.encoding_contexts.remove(&pdu.codec_context_id).expect("missing encoding context");
    }

    /****** FRAME MANAGEMENT ******/
    fn on_frame_complete(&mut self, _frame_id: u32) {
        let mapped_surfaces: Vec<MappedSurface<S>> = self.surfaces.iter().filter_map(|(_, surface)|
            surface.mapping.as_ref().map(|m| MappedSurface{
                surface: &surface.surface,
                mapping: m.clone(),
            })
        ).collect();

        (self.draw_cb)(&mapped_surfaces)
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
