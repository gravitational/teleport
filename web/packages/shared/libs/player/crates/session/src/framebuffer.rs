//! IronRDP FastPath processor + framebuffer.
//!
//! One IronRDP processor produces one full-virtual-desktop `DecodedImage`.
//! Each registered canvas paints a viewport-shaped slice of that image —
//! this lets a single decoder drive any number of multi-monitor windows
//! without duplicating decode work.

use std::collections::HashMap;

use anyhow::{anyhow, bail, Result};
use ironrdp_core::{decode_cursor, ReadCursor, WriteBuf};
use ironrdp_graphics::image_processing::PixelFormat;
use ironrdp_pdu::fast_path::{FastPathHeader, FastPathUpdatePdu, UpdateCode};
use ironrdp_pdu::geometry::InclusiveRectangle;
use ironrdp_session::fast_path::{
    Processor as IronRdpFastPathProcessor, ProcessorBuilder as IronRdpFastPathProcessorBuilder,
    UpdateKind,
};
use ironrdp_session::image::DecodedImage;
use web_sys::{OffscreenCanvas, VideoFrame};

use crate::gl::GlPainter;
use crate::perf::{self, PduClass};
use ironrdp_graphics::progressive::ProgressiveDecoder;

fn classify_pdu(pdu: &[u8]) -> PduClass {
    let mut input = ReadCursor::new(pdu);
    if decode_cursor::<FastPathHeader>(&mut input).is_err() {
        return PduClass::Other;
    }
    let update = match decode_cursor::<FastPathUpdatePdu<'_>>(&mut input) {
        Ok(u) => u,
        Err(_) => return PduClass::Other,
    };
    match update.update_code {
        UpdateCode::SurfaceCommands => PduClass::SurfaceCommands,
        UpdateCode::Bitmap => PduClass::Bitmap,
        UpdateCode::HiddenPointer
        | UpdateCode::DefaultPointer
        | UpdateCode::PositionPointer
        | UpdateCode::ColorPointer
        | UpdateCode::CachedPointer
        | UpdateCode::NewPointer => PduClass::Pointer,
        _ => PduClass::Other,
    }
}

#[derive(Debug, Clone, Copy)]
struct DirtyRect {
    left: u32,
    top: u32,
    right: u32,
    bottom: u32,
}

impl DirtyRect {
    fn union(self, o: DirtyRect) -> DirtyRect {
        DirtyRect {
            left: self.left.min(o.left),
            top: self.top.min(o.top),
            right: self.right.max(o.right),
            bottom: self.bottom.max(o.bottom),
        }
    }

    /// Half-open `[left,right) x [top,bottom)` intersection test. Adjacent
    /// (edge-touching) rects do NOT overlap.
    fn overlaps(self, o: DirtyRect) -> bool {
        self.left < o.right && o.left < self.right && self.top < o.bottom && o.top < self.bottom
    }

    fn area(self) -> u64 {
        u64::from(self.right.saturating_sub(self.left))
            * u64::from(self.bottom.saturating_sub(self.top))
    }
}

/// Cap on the number of disjoint dirty rectangles tracked per frame. EGFX
/// stamps (especially `CacheToSurface`, the dominant op) paint many small,
/// scattered regions; keeping them as separate rects means `render` uploads
/// only the changed pixels instead of one near-fullscreen bounding box. When
/// the cap is exceeded we union the new rect into the existing one whose area
/// grows least — coverage stays a *superset* of every rect ever added, so a
/// region is never dropped; overflow only costs extra (over-)upload.
const MAX_DIRTY_RECTS: usize = 16;

pub enum CursorEvent {
    Bitmap {
        rgba: Vec<u8>,
        width: u16,
        height: u16,
        hotspot_x: i32,
        hotspot_y: i32,
    },
    Hidden,
    Default,
}

pub struct ProcessOutput {
    pub response: Vec<u8>,
    pub cursors: Vec<CursorEvent>,
}

#[derive(Clone, Copy, Debug)]
pub struct Viewport {
    pub x: u32,
    pub y: u32,
    pub width: u32,
    pub height: u32,
}

/// One registered canvas: its painter and the viewport it sees. The viewport
/// band is uploaded straight from the framebuffer (no per-canvas scratch).
struct CanvasView {
    painter: GlPainter,
    /// The viewport actually in effect — `requested` clamped to the current
    /// framebuffer dims. This is what `render` reads.
    viewport: Viewport,
    /// The viewport the caller asked for, BEFORE clamping. A canvas can be
    /// registered/repositioned while the framebuffer is still at its old
    /// (smaller) size — e.g. a new monitor is added and its canvas registered
    /// before the server's ResetGraphics grows the buffer — which clamps its
    /// visible region down to a sliver. Keeping the un-clamped request lets a
    /// later grow (`resize_preserving_canvases`) restore the full intended
    /// slice instead of permanently inheriting the clamped-against-old-dims
    /// value.
    requested: Viewport,
}

impl CanvasView {
    fn new(
        canvas: &OffscreenCanvas,
        image_width: u16,
        image_height: u16,
        viewport: Viewport,
    ) -> Result<Self> {
        let requested = viewport;
        let viewport = clamp_viewport(viewport, image_width, image_height);
        let painter = GlPainter::new(canvas, viewport.width, viewport.height)?;
        Ok(Self { painter, viewport, requested })
    }
}

fn clamp_viewport(v: Viewport, image_width: u16, image_height: u16) -> Viewport {
    let iw = u32::from(image_width);
    let ih = u32::from(image_height);
    let x = v.x.min(iw);
    let y = v.y.min(ih);
    Viewport {
        x,
        y,
        width: v.width.min(iw.saturating_sub(x)).max(1),
        height: v.height.min(ih.saturating_sub(y)).max(1),
    }
}

/// Build the IronRDP fast-path processor for a (re)activation. share_id only
/// matters when sending FrameMarker responses; the wasm decoder never sends
/// back, so 0 is safe.
fn build_fast_path(io_channel_id: u16, user_channel_id: u16) -> IronRdpFastPathProcessor {
    IronRdpFastPathProcessorBuilder {
        io_channel_id,
        user_channel_id,
        share_id: 0,
        enable_server_pointer: true,
        pointer_software_rendering: false,
        // Bulk compression not negotiated for our session.
        bulk_decompressor: None,
    }
    .build()
}

/// A fresh framebuffer image filled with opaque black. ClearCodec's wire format
/// assumes the client framebuffer already holds a valid previous-frame image —
/// partial-coverage PDUs explicitly preserve whatever pixels were there before.
/// For a fresh connection (or a fresh wasm load mid-session), Windows relies on
/// the client starting with a black backdrop matching the server-side desktop
/// default; without this fill, un-painted regions appear as `DecodedImage`'s
/// zero-fill (RGBA 0,0,0,0) and visibly diverge from what mstsc/FreeRDP shows.
fn new_init_image(width: u16, height: u16) -> DecodedImage {
    let mut image = DecodedImage::new(PixelFormat::RgbA32, width, height);
    for px in image.data_mut().chunks_exact_mut(4) {
        px[0] = 0x00;
        px[1] = 0x00;
        px[2] = 0x00;
        px[3] = 0xff;
    }
    image
}

pub struct Framebuffer {
    fast_path: IronRdpFastPathProcessor,
    image: DecodedImage,
    width: u16,
    height: u16,
    /// Disjoint dirty rectangles accumulated since the last render (capped at
    /// `MAX_DIRTY_RECTS`). `render` uploads each one separately so scattered
    /// EGFX stamps don't inflate into a single near-fullscreen `texSubImage`.
    dirty: Vec<DirtyRect>,
    /// Set by EGFX handlers via `mark_dirty_pending`; consumed by
    /// `flush_render`. Lets `feed_bytes` coalesce many PDUs in one wire
    /// burst into a single GL upload by scheduling exactly one
    /// `setTimeout(0)` flush per burst instead of rendering per PDU.
    dirty_pending: bool,
    /// Canvases keyed by caller-assigned id. `Vec` (not `HashMap`) because
    /// N is tiny (≤ 3 in practice) and ordered iteration is convenient.
    canvases: Vec<(u32, CanvasView)>,
    /// One stateful ClearCodec decoder per EGFX surface. The glyph + vbar
    /// caches inside each decoder must survive across PDUs belonging to the
    /// same surface; new surfaces get a fresh decoder lazy-allocated on
    /// first PDU. `Vec` matches `canvases` — N is small (1–3 surfaces).
    /// One shared ClearCodec decoder for the whole connection. FreeRDP keeps a
    /// single CLEAR_CONTEXT; the glyph + V-bar caches are connection-scoped
    /// (indexed only by glyphIndex / vbar index, never by surface), so a
    /// per-surface split corrupts cross-surface cache hits.
    clearcodec_decoder: ironrdp_graphics::clearcodec::ClearCodecDecoder,
    /// One stateful RFX Progressive decoder per EGFX surface. Per-tile
    /// coefficient state lives inside the decoder, keyed by
    /// `codec_context_id`; `DeleteEncodingContext` evicts a single context
    /// without dropping the whole surface's state.
    progressive_decoders: Vec<(u32, ProgressiveDecoder)>,
    /// EGFX bitmap cache keyed by `cache_slot` ([MS-RDPEGFX] 2.2.2.6 et al).
    /// Each entry holds the RGBA snapshot of a surface region copied via
    /// `SurfaceToCache`; later `CacheToSurface` PDUs blit it back at one or
    /// more destination points. Windows uses this for tiled UI panels,
    /// taskbar segments, repeated icons, etc. — so without it, large
    /// stretches of the screen remain at the framebuffer's init color.
    bitmap_cache: HashMap<u32, BitmapCacheEntry>,
    /// Reused scratch for `apply_surface_to_surface` snapshots: the source
    /// region is copied out before blitting to the dest points (src/dst alias
    /// `image`), so the copy is transient and the buffer can be pooled.
    s2s_scratch: Vec<u8>,
}

struct BitmapCacheEntry {
    width: u32,
    height: u32,
    rgba: Vec<u8>,
}

// DIAGNOSTIC (revert): pixel write-history probe (Detection A). The black rects
// are scattered everywhere, so we sample a GRID of framebuffer points (coords
// are 2x logical at 200% scale; framebuffer is 3200x660). `probe_after` logs
// `[pixhist] (x,y) op=… -> (r,g,b,a)` after any paint whose dest rect covers a
// sample point; `apply_surface_to_cache` also logs the value at SNAPSHOT time.
// So per point we see the full order — content paints, the cache snapshot, and
// the cache replays — which tells us why a given tile ends black.
const PROBES: &[(u32, u32)] = &[
    (16, 16), (16, 330), (16, 650), // INSIDE the hot bg-fill SOURCE tiles (slot 2/5 = (0,0)-(64,64))
    (400, 30), (1600, 30), (2800, 30), // top chrome (title bars)
    (400, 330), (1600, 330), (2800, 330), // middle (content)
    (400, 630), (1600, 630), (2800, 630), // bottom chrome (taskbar)
];

impl Framebuffer {
    pub fn new(io_channel_id: u16, user_channel_id: u16, width: u16, height: u16) -> Result<Self> {
        let fast_path = build_fast_path(io_channel_id, user_channel_id);
        let image = new_init_image(width, height);
        Ok(Self {
            fast_path,
            image,
            width,
            height,
            dirty: Vec::new(),
            dirty_pending: false,
            canvases: Vec::new(),
            clearcodec_decoder: ironrdp_graphics::clearcodec::ClearCodecDecoder::new(),
            progressive_decoders: Vec::new(),
            bitmap_cache: HashMap::new(),
            s2s_scratch: Vec::new(),
        })
    }

    pub fn width(&self) -> u16 {
        self.width
    }

    pub fn height(&self) -> u16 {
        self.height
    }

    pub fn add_canvas(&mut self, id: u32, canvas: &OffscreenCanvas, viewport: Viewport) -> Result<()> {
        let view = CanvasView::new(canvas, self.width, self.height, viewport)?;
        // Replace any existing entry with the same id (caller bug, but be
        // defensive — orphaning a painter could leak GL state).
        self.remove_canvas(id);
        self.canvases.push((id, view));
        // Mark the whole viewport dirty so the new canvas gets a full first
        // paint instead of a black frame waiting for the next PDU.
        self.mark_all_dirty();
        Ok(())
    }

    pub fn remove_canvas(&mut self, id: u32) {
        self.canvases.retain(|(cid, _)| *cid != id);
    }

    pub fn update_viewport(
        &mut self,
        id: u32,
        canvas: &OffscreenCanvas,
        viewport: Viewport,
    ) -> Result<()> {
        // Easiest correct path: rebuild the CanvasView. GL state is per-painter
        // and a viewport change implies new dims, so reusing the old painter
        // would require resizing its backing texture/quad anyway.
        let view = CanvasView::new(canvas, self.width, self.height, viewport)?;
        if let Some(entry) = self.canvases.iter_mut().find(|(cid, _)| *cid == id) {
            entry.1 = view;
        } else {
            self.canvases.push((id, view));
        }
        // Force a full repaint of the new viewport on the next render.
        self.mark_all_dirty();
        Ok(())
    }

    /// Re-point an already-registered canvas at a new viewport WITHOUT needing
    /// the `OffscreenCanvas` re-passed (it was consumed by `add_canvas` and the
    /// caller no longer holds it). Used when a monitor is removed and the
    /// survivors reflow into new desktop positions: typically only the source
    /// offset changes, so the painter is reused and only resized when its
    /// texture dims actually change (gl.rs recovers the canvas from its own GL
    /// context, so no handle is needed).
    pub fn reposition_canvas(&mut self, id: u32, viewport: Viewport) -> Result<()> {
        let clamped = clamp_viewport(viewport, self.width, self.height);
        if let Some((_, view)) = self.canvases.iter_mut().find(|(cid, _)| *cid == id) {
            view.requested = viewport;
            if clamped.width != view.viewport.width || clamped.height != view.viewport.height {
                view.painter.resize(clamped.width, clamped.height)?;
            }
            view.viewport = clamped;
            self.mark_all_dirty();
        }
        Ok(())
    }

    /// Resize the framebuffer to a new desktop size for a mid-session
    /// DisplayControl layout change (monitor added/removed/moved) while KEEPING
    /// the registered output canvases, the already-painted content, the EGFX
    /// caches, AND the fast-path processor (with its server pointer cache).
    /// `init_framebuffer` otherwise creates a fresh `Framebuffer` on any
    /// dimension change, which drops every canvas and blanks the screen — fatal
    /// once the session resizes mid-flight (e.g. a popup monitor is closed).
    ///
    /// Unlike a full RDP reactivation, the server does NOT repaint the whole
    /// desktop after a DisplayControl resize — it sends only incremental
    /// updates, and the channel IDs / surfaces / pointer cache all survive. So
    /// this preserves the overlapping image region, the connection-scoped EGFX
    /// cache/decoder state, and the fast-path processor rather than rebuilding
    /// from scratch; otherwise unchanged regions and cache replays (pixels OR
    /// cursor bitmaps) render as black / wrong / missing. Each kept canvas's
    /// viewport is re-derived (from its un-clamped request) against the new
    /// dims; callers may additionally reposition survivors via `reposition_canvas`.
    pub fn resize_preserving_canvases(&mut self, width: u16, height: u16) -> Result<()> {
        // Keep the fast-path processor as-is. A DisplayControl resize is NOT a
        // reactivation: the io/user channel IDs are unchanged, and the processor
        // owns the SERVER POINTER CACHE (cursor bitmaps keyed by cache slot —
        // ironrdp-session `Processor::pointer_cache`). Rebuilding it would empty
        // that cache, so the next `CachedPointer` PDU referencing a slot
        // populated before the resize misses ("Cached pointer not found") and
        // the cursor shows a stale/wrong bitmap or vanishes. `build_fast_path`
        // takes no dimensions, so a resize never needs to rebuild it.
        //
        // Preserve the already-painted desktop. A mid-session DisplayControl
        // resize (monitor added/removed/moved) grows or shrinks the framebuffer,
        // but the server only sends incremental/dirty-region updates afterward —
        // it does NOT repaint the existing desktop. Blanking here would leave
        // every region the server considers unchanged black, and would turn
        // later `CacheToSurface` replays into cold-miss black rects. So copy the
        // overlapping region into the new buffer rather than start from black.
        //
        // Origin is (0,0): the server anchors the primary monitor's content at
        // the framebuffer origin and keeps it there across a layout change, so
        // existing content stays valid at the same coordinates (the common
        // add/remove-to-the-right/below case). A layout whose bounding-box origin
        // shifts — a monitor added left/above the primary — is a separate,
        // known limitation tracked elsewhere; it needs the client viewport
        // origin to match the server's primary-anchored composition origin.
        let old_w = usize::from(self.width);
        let old_h = usize::from(self.height);
        let mut new_image = new_init_image(width, height);
        {
            let new_w = usize::from(width);
            let copy_w = old_w.min(new_w);
            let copy_h = old_h.min(usize::from(height));
            if copy_w > 0 && copy_h > 0 {
                let src = self.image.data();
                let dst = new_image.data_mut();
                let src_stride = old_w * 4;
                let dst_stride = new_w * 4;
                let row_bytes = copy_w * 4;
                for row in 0..copy_h {
                    let s = row * src_stride;
                    let d = row * dst_stride;
                    dst[d..d + row_bytes].copy_from_slice(&src[s..s + row_bytes]);
                }
            }
        }
        self.image = new_image;
        self.width = width;
        self.height = height;
        self.mark_all_dirty();
        self.dirty_pending = false;
        // Keep the EGFX decode/cache state: a DisplayControl resize is NOT a
        // reactivation. The bitmap cache is connection-scoped and survives a
        // ResetGraphics layout change ([MS-RDPEGFX] — the server evicts entries
        // only via EvictCacheEntry), and the ClearCodec / progressive contexts
        // carry per-surface state the server keeps reusing. Clearing them here
        // is exactly what turned post-resize cache replays into black rects.
        //
        // Keep the output canvases; re-clamp each viewport to the new dims from
        // the un-clamped REQUEST (not the current viewport, which may have been
        // clamped against the old smaller buffer when the canvas was registered
        // before the grow landed), and resize any painter whose visible size
        // actually changed.
        for (_, view) in self.canvases.iter_mut() {
            let clamped = clamp_viewport(view.requested, width, height);
            if clamped.width != view.viewport.width || clamped.height != view.viewport.height {
                view.painter.resize(clamped.width, clamped.height)?;
            }
            view.viewport = clamped;
        }
        Ok(())
    }

    /// Blit pre-decoded RGBA bytes into the framebuffer image at the given
    /// desktop top-left, then expand the dirty rectangle so the next
    /// `render()` repaints any canvas whose viewport overlaps. Used by the
    /// EGFX path — pixels are already in RGBA8 desktop-coordinate form.
    /// DIAGNOSTIC (revert): log one sample point's RGBA, tagged with the op that
    /// just wrote it. `[pixhist]` is consumed by detector #1 in logsink.ts.
    fn probe_pixel(&self, op: &str, px: u32, py: u32) {
        let w = u32::from(self.width);
        let h = u32::from(self.height);
        if px >= w || py >= h {
            return;
        }
        let i = (py as usize * w as usize + px as usize) * 4;
        let d = self.image.data();
        let _ = (op, &d, i); // DIAGNOSTIC (revert): muted; un-comment the warn below to re-enable.
        if i + 3 < d.len() {
            // web_sys::console::warn_1(
            //     &format!(
            //         "[pixhist] ({px},{py}) op={op} -> ({},{},{},{})",
            //         d[i], d[i + 1], d[i + 2], d[i + 3]
            //     )
            //     .into(),
            // );
        }
    }

    /// DIAGNOSTIC (revert): log every sample point whose covering rect [l,r)x[t,b)
    /// contains it, so per point we get the exact ordered op sequence.
    fn probe_after(&self, op: &str, l: u32, t: u32, r: u32, b: u32) {
        for &(px, py) in PROBES {
            if px >= l && px < r && py >= t && py < b {
                self.probe_pixel(op, px, py);
            }
        }
    }

    pub fn blit_rgba(&mut self, dst_x: u32, dst_y: u32, w: u32, h: u32, rgba: &[u8]) -> Result<()> {
        if w == 0 || h == 0 {
            return Ok(());
        }
        let expected = (w as usize)
            .checked_mul(h as usize)
            .and_then(|n| n.checked_mul(4))
            .ok_or_else(|| anyhow!("egfx rgba size overflow"))?;
        if rgba.len() != expected {
            bail!(
                "egfx rgba size mismatch: expected {} bytes for {}x{}, got {}",
                expected,
                w,
                h,
                rgba.len()
            );
        }

        let img_w = u32::from(self.width);
        let img_h = u32::from(self.height);
        if dst_x >= img_w || dst_y >= img_h {
            // Fully off-image; ignore rather than panic (similar to
            // upstream PR #1146's skip-OOB-bitmap behavior).
            return Ok(());
        }
        let max_w = (img_w - dst_x).min(w);
        let max_h = (img_h - dst_y).min(h);
        let src_stride = (w as usize) * 4;
        let dst_stride = (img_w as usize) * 4;
        let dst = self.image.data_mut();
        for row in 0..max_h as usize {
            let src_off = row * src_stride;
            let dst_off = ((dst_y as usize) + row) * dst_stride + (dst_x as usize) * 4;
            let copy_len = (max_w as usize) * 4;
            dst[dst_off..dst_off + copy_len]
                .copy_from_slice(&rgba[src_off..src_off + copy_len]);
        }

        let rect = DirtyRect {
            left: dst_x,
            top: dst_y,
            right: (dst_x + max_w).min(img_w),
            bottom: (dst_y + max_h).min(img_h),
        };
        self.probe_after("blit_rgba", rect.left, rect.top, rect.right, rect.bottom);
        self.add_dirty_rect(rect);
        Ok(())
    }

    /// AVC fast path: upload a decoded `VideoFrame` straight to the GPU
    /// texture(s) of any canvas that fully shows the video rect, GPU→GPU with
    /// no readback. Returns `true` if drawn, `false` if the caller should fall
    /// back to the CPU `blit_rgba` path (e.g. the video spans a monitor edge,
    /// which `tex_sub_image(VideoFrame)` can't crop). Bypasses the CPU
    /// framebuffer entirely, so the video and the (CPU-composited) chrome write
    /// disjoint regions of the same texture.
    pub fn blit_video_frame(
        &self,
        desktop_x: u32,
        desktop_y: u32,
        frame: &VideoFrame,
    ) -> Result<bool> {
        let vw = frame.display_width();
        let vh = frame.display_height();
        if vw == 0 || vh == 0 {
            return Ok(false);
        }
        let vx1 = desktop_x.saturating_add(vw);
        let vy1 = desktop_y.saturating_add(vh);

        let mut drawn = false;
        for (_, view) in self.canvases.iter() {
            let vp = view.viewport;
            // Skip canvases the video doesn't touch.
            let ix0 = desktop_x.max(vp.x);
            let iy0 = desktop_y.max(vp.y);
            let ix1 = vx1.min(vp.x + vp.width);
            let iy1 = vy1.min(vp.y + vp.height);
            if ix0 >= ix1 || iy0 >= iy1 {
                continue;
            }
            // The direct upload can't crop the source, so the whole video must
            // fit inside this canvas. If it spills the viewport, bail to the CPU
            // fallback for the entire blit (don't half-upload).
            if desktop_x < vp.x
                || desktop_y < vp.y
                || vx1 > vp.x + vp.width
                || vy1 > vp.y + vp.height
            {
                return Ok(false);
            }
            let dst_x = (desktop_x - vp.x) as i32;
            let dst_y = (desktop_y - vp.y) as i32;
            view.painter
                .upload_video_frame_and_draw(frame, dst_x, dst_y, vw as i32, vh as i32)?;
            drawn = true;
        }
        Ok(drawn)
    }

    /// Decode an EGFX ClearCodec PDU via upstream `ironrdp_graphics` and blit
    /// the result into the framebuffer at `(dest_x, dest_y)`. The upstream
    /// decoder returns a BGRA buffer; `blit_bgra_clipped` handles the B↔R
    /// swap, alpha forcing, and edge-clipping so negative / out-of-bounds
    /// destinations are silently clipped rather than rejected.
    pub fn apply_clearcodec(
        &mut self,
        surface_id: u32,
        dest_x: i32,
        dest_y: i32,
        w: u32,
        h: u32,
        pdu: &[u8],
    ) -> Result<()> {
        let dest_w = u16::try_from(w).map_err(|_| anyhow!("clearcodec width {w} > u16"))?;
        let dest_h = u16::try_from(h).map_err(|_| anyhow!("clearcodec height {h} > u16"))?;
        if dest_w == 0 || dest_h == 0 {
            return Ok(());
        }

        let _ = surface_id; // ClearCodec caches are connection-scoped, not per-surface
        // Decode (the half that moves to the dedicated decode worker in Step 2).
        let bgra = self
            .clearcodec_decoder
            .decode(pdu, dest_w, dest_h)
            .map_err(|e| anyhow!("clearcodec decode failed: {e}"))?;

        // DIAGNOSTIC (revert): muted — counted covered-black vs uncovered in the
        // decoded tile and logged [cc-black]; un-comment to re-enable.
        /*
        // DIAGNOSTIC (revert): attribute black rects. Count covered-black
        // (alpha 0xFF & RGB 0,0,0 — actively painted black) vs uncovered
        // (alpha 0 — preserved prior). A high covered_black% means ClearCodec
        // itself decoded this tile to black.
        {
            let total = bgra.len() / 4;
            if total > 0 {
                let mut cblack = 0usize;
                let mut uncov = 0usize;
                for px in bgra.chunks_exact(4) {
                    if px[3] == 0 {
                        uncov += 1;
                    } else if px[0] == 0 && px[1] == 0 && px[2] == 0 {
                        cblack += 1;
                    }
                }
                let cbp = cblack * 100 / total;
                if cbp >= 20 {
                    let (gly, res, band, sub) = self.clearcodec_decoder.last_layers;
                    let (sraw, srlex, sns) = self.clearcodec_decoder.last_sub;
                    let (ns_cll, ns_chroma) = self.clearcodec_decoder.last_ns_hdr;
                    web_sys::console::warn_1(
                        &format!(
                            "[cc-black] dest=({dest_x},{dest_y}) {dest_w}x{dest_h} covered_black={cbp}% uncovered={}% layers=glyph{}/residual{}/bands{}/subcodec{} sub=raw{}/rlex{}/ns{} ns_hdr=cll{}/chroma{}",
                            uncov * 100 / total,
                            u8::from(gly), u8::from(res), u8::from(band), u8::from(sub),
                            u8::from(sraw), u8::from(srlex), u8::from(sns),
                            ns_cll, ns_chroma,
                        )
                        .into(),
                    );
                }
            }
        }
        */

        self.blit_clear_bgra(dest_x, dest_y, dest_w, dest_h, &bgra);
        Ok(())
    }

    /// Blit a decoded ClearCodec BGRA buffer into the framebuffer image at
    /// `(dest_x, dest_y)` (the upstream decoder emits BGRA; `blit_bgra_clipped`
    /// handles the B↔R swap, alpha forcing, and edge-clipping so negative /
    /// out-of-bounds destinations are silently clipped), then mark the
    /// destination rectangle dirty. The decode half (`ClearCodecDecoder::decode`)
    /// moves to the Step 2 decode worker; this blit half stays on the canvas
    /// worker and is shared by the inline path and the decode-worker return path.
    pub(crate) fn blit_clear_bgra(
        &mut self,
        dest_x: i32,
        dest_y: i32,
        dest_w: u16,
        dest_h: u16,
        bgra: &[u8],
    ) {
        let img_w = self.image.width() as u32;
        let img_h = self.image.height() as u32;
        blit_bgra_clipped(
            self.image.data_mut(),
            img_w,
            img_h,
            dest_x,
            dest_y,
            u32::from(dest_w),
            u32::from(dest_h),
            bgra,
        );

        // Coarse dirty rect: the entire destination rectangle. Edge-clipped
        // pixels that fall outside the image are excluded via the saturating
        // clamp below so we never mark a dirty region outside the image.
        let left = dest_x.max(0) as u32;
        let top = dest_y.max(0) as u32;
        let right = ((dest_x + i32::from(dest_w)).max(0) as u32).min(img_w);
        let bottom = ((dest_y + i32::from(dest_h)).max(0) as u32).min(img_h);
        self.probe_after("clearcodec", left, top, right, bottom);
        if right > left && bottom > top {
            let rect = DirtyRect {
                left,
                top,
                right,
                bottom,
            };
            self.add_dirty_rect(rect);
        }
    }

    /// Apply an EGFX `Codec1Type::Planar` PDU. Decodes the RDP 6.0 bitmap
    /// stream into RGB24 via `ironrdp_graphics::rdp6::bitmap_stream`, then
    /// expands to RGBA32 (alpha = 0xFF) and blits at `(dest_x, dest_y)`.
    pub fn apply_planar(
        &mut self,
        _surface_id: u32,
        dest_x: i32,
        dest_y: i32,
        w: u32,
        h: u32,
        pdu: &[u8],
    ) -> Result<()> {
        if w == 0 || h == 0 {
            return Ok(());
        }
        if dest_x < 0 || dest_y < 0 {
            bail!("planar dest ({dest_x},{dest_y}) is negative");
        }
        let dest_x = dest_x as u32;
        let dest_y = dest_y as u32;
        let img_w = u32::from(self.width);
        let img_h = u32::from(self.height);
        if dest_x >= img_w || dest_y >= img_h {
            return Ok(());
        }
        let cw = w.min(img_w - dest_x);
        let ch = h.min(img_h - dest_y);

        let mut rgb24: Vec<u8> = Vec::new();
        let mut dec = ironrdp_graphics::rdp6::BitmapStreamDecoder::default();
        dec.decode_bitmap_stream_to_rgb24(pdu, &mut rgb24, w as usize, h as usize)
            .map_err(|e| anyhow!("planar decode failed: {e}"))?;
        if rgb24.len() < (w as usize) * (h as usize) * 3 {
            bail!(
                "planar decoder underran: produced {} bytes for {}x{} (expected {})",
                rgb24.len(), w, h, (w as usize) * (h as usize) * 3,
            );
        }

        let src_row_stride = w as usize * 3;
        let dst_row_stride = img_w as usize * 4;
        let copy_w = cw as usize;
        let buf = self.image.data_mut();
        for row in 0..ch as usize {
            let src_off = row * src_row_stride;
            let dst_off = (dest_y as usize + row) * dst_row_stride + dest_x as usize * 4;
            for col in 0..copy_w {
                let s = &rgb24[src_off + col * 3..src_off + col * 3 + 3];
                let d = &mut buf[dst_off + col * 4..dst_off + col * 4 + 4];
                d[0] = s[0];
                d[1] = s[1];
                d[2] = s[2];
                d[3] = 0xFF;
            }
        }
        self.probe_after("planar", dest_x, dest_y, dest_x + cw, dest_y + ch);
        let rect = DirtyRect {
            left: dest_x,
            top: dest_y,
            right: dest_x + cw,
            bottom: dest_y + ch,
        };
        self.add_dirty_rect(rect);
        Ok(())
    }

    /// Apply an EGFX `Codec1Type::Uncompressed` PDU. `pixel_format` is the
    /// MS-RDPEGFX byte: `0x20` = PIXEL_FORMAT_XRGB_8888 (alpha byte is
    /// padding — overwrite), `0x21` = PIXEL_FORMAT_ARGB_8888 (alpha byte
    /// is meaningful — source-over composite). In both cases the wire
    /// layout is `[B, G, R, X/A]` per pixel (little-endian uint32
    /// `0x[XX|AA]RRGGBB`); our destination framebuffer is RGBA32 so we
    /// reorder while blitting and always leave destination alpha = 0xFF.
    pub fn apply_uncompressed(
        &mut self,
        _surface_id: u32,
        dest_x: i32,
        dest_y: i32,
        w: u32,
        h: u32,
        pixel_format: u32,
        bitmap: &[u8],
    ) -> Result<()> {
        if w == 0 || h == 0 {
            return Ok(());
        }
        if dest_x < 0 || dest_y < 0 {
            bail!("uncompressed dest ({dest_x},{dest_y}) is negative");
        }
        let dest_x = dest_x as u32;
        let dest_y = dest_y as u32;
        let img_w = u32::from(self.width);
        let img_h = u32::from(self.height);
        if dest_x >= img_w || dest_y >= img_h {
            return Ok(());
        }
        // Clip to image bounds.
        let cw = w.min(img_w - dest_x);
        let ch = h.min(img_h - dest_y);
        let expected = (w as usize) * (h as usize) * 4;
        if bitmap.len() < expected {
            bail!(
                "uncompressed payload short: {}x{} needs {} bytes, got {}",
                w, h, expected, bitmap.len()
            );
        }

        // Treat both XRGB_8888 (0x20) and ARGB_8888 (0x21) as opaque BGRA.
        // The pixel_format byte describes wire layout, NOT a request to do
        // alpha compositing — mstsc/FreeRDP write Uncompressed pixels
        // verbatim and force destination alpha to 0xFF. The previous
        // source-over branch was misreading 0x21 as "blend me", which
        // produced washed-out gray-over-icon artifacts on the Win11
        // taskbar (its hover backgrounds arrive as ARGB with alpha < 255).
        let _ = pixel_format;
        let stride = img_w as usize * 4;
        let src_stride = w as usize * 4;
        let buf = self.image.data_mut();
        for row in 0..ch as usize {
            let src_row = row * src_stride;
            let dst_row = (dest_y as usize + row) * stride + dest_x as usize * 4;
            for col in 0..cw as usize {
                let s = &bitmap[src_row + col * 4..src_row + col * 4 + 4];
                // Wire bytes are [B, G, R, X/A] (little-endian 0xAARRGGBB).
                let d = &mut buf[dst_row + col * 4..dst_row + col * 4 + 4];
                d[0] = s[2];
                d[1] = s[1];
                d[2] = s[0];
                d[3] = 0xFF;
            }
        }
        // DIAGNOSTIC (revert): muted — painted a blue corner marker on Uncompressed
        // regions; un-comment to re-enable.
        /*
        // DIAGNOSTIC (revert): tag Uncompressed regions with a blue corner.
        {
            let n = (cw as usize).min(8);
            let m = (ch as usize).min(8);
            for r in 0..m {
                let dr = (dest_y as usize + r) * stride + dest_x as usize * 4;
                for c in 0..n {
                    let d = &mut buf[dr + c * 4..dr + c * 4 + 4];
                    d[0] = 0x00;
                    d[1] = 0x00;
                    d[2] = 0xFF;
                    d[3] = 0xFF;
                }
            }
        }
        */
        self.probe_after("uncompressed", dest_x, dest_y, dest_x + cw, dest_y + ch);
        let rect = DirtyRect {
            left: dest_x,
            top: dest_y,
            right: dest_x + cw,
            bottom: dest_y + ch,
        };
        self.add_dirty_rect(rect);
        Ok(())
    }

    /// Decode an RFX Progressive payload (`WireToSurface2`) and blit each
    /// produced tile into the framebuffer image at
    /// `(surface_origin + tile.dest_x/y)`. Per-(surface, codec_context_id)
    /// state lives in `progressive_decoders[surface_id]` and survives across
    /// PDUs so TileUpgrade refinement chains can land later.
    ///
    /// `pixel_format` is ignored — the decoder produces opaque RGBA and we
    /// blit straight into the framebuffer's RGBA32 image. Tiles that fall
    /// (partially) outside the image are clipped row/col-wise.
    pub fn apply_rfx_progressive(
        &mut self,
        surface_id: u32,
        codec_context_id: u32,
        surface_origin_x: u32,
        surface_origin_y: u32,
        payload: &[u8],
    ) -> Result<()> {
        // Install the wasm millisecond clock into ironrdp-graphics' stage timer
        // once per worker, then zero the per-decode accumulator so `take()`
        // below reads only this PDU's entropy/IDWT/color time.
        #[cfg(feature = "rfx-stage-timing")]
        {
            use std::cell::Cell;
            thread_local! {
                static HOOK_SET: Cell<bool> = const { Cell::new(false) };
            }
            HOOK_SET.with(|c| {
                if !c.get() {
                    ironrdp_graphics::progressive::stage_timing::set_now_hook(crate::perf::performance_now);
                    c.set(true);
                }
            });
            ironrdp_graphics::progressive::stage_timing::reset();
        }

        // Decode (the half that moves to the dedicated decode worker in Step 2).
        // Scope the decoder borrow so the blit below can take `&mut self`.
        let tiles = {
            let dec = lookup_or_create_progressive(&mut self.progressive_decoders, surface_id);
            match dec.decode_bitmap(codec_context_id, self.width, self.height, payload) {
                Ok(t) => t,
                Err(e) => {
                    // Reset on a mid-frame decode error so partially-advanced tile
                    // accumulators can't poison (and accumulate across) future
                    // frames; resyncs on the next keyframe. Mirrors the offload
                    // decode worker (`egfx_decoder::decode_wire_to_surface2`).
                    dec.reset();
                    return Err(anyhow!("rfx-progressive decode failed: {e}"));
                }
            }
        };
        if tiles.is_empty() {
            return Ok(());
        }

        // Blit (the half that stays on the canvas/GL worker).
        #[cfg(feature = "rfx-stage-timing")]
        let blit_start = crate::perf::performance_now();
        self.blit_rfx_tiles(surface_origin_x, surface_origin_y, &tiles);
        #[cfg(feature = "rfx-stage-timing")]
        {
            let blit_ms = crate::perf::performance_now() - blit_start;
            let st = ironrdp_graphics::progressive::stage_timing::take();
            crate::perf::record_rfx_stages(st.entropy_ms, st.idwt_ms, st.color_ms, blit_ms);
        }

        // Recycle the tile pixel buffers into the decoder's pool for reuse on the
        // next frame (avoids a 16 KB alloc per tile per frame). Re-borrow the
        // decoder (linear scan over ≤3 surfaces) now that the blit's `&mut self`
        // borrow has ended.
        lookup_or_create_progressive(&mut self.progressive_decoders, surface_id).reclaim(tiles);
        Ok(())
    }

    /// Composite decoded RFX-Progressive tiles into the framebuffer image,
    /// clipped per tile `clip_rects` (FreeRDP `update_tiles` semantics: only the
    /// damaged region is repainted and prior-frame content is preserved
    /// elsewhere — blitting whole tiles over-paints partial-region tiles,
    /// verified divergent vs FreeRDP), and mark the painted union dirty.
    ///
    /// The decode half lives in [`ProgressiveDecoder::decode_bitmap`]; this blit
    /// half is shared by the inline [`Self::apply_rfx_progressive`] path and the
    /// Step 2 decode-worker return path (tiles arriving over `postMessage`).
    pub(crate) fn blit_rfx_tiles(
        &mut self,
        surface_origin_x: u32,
        surface_origin_y: u32,
        tiles: &[ironrdp_graphics::progressive::DecodedTile],
    ) {
        let img_w = u32::from(self.width);
        let img_h = u32::from(self.height);
        let stride = img_w as usize * 4;
        let buf = self.image.data_mut();
        let mut union: Option<DirtyRect> = None;

        for t in tiles {
            composite_rfx_tile(
                buf,
                stride,
                img_w,
                img_h,
                surface_origin_x,
                surface_origin_y,
                t.x_idx,
                t.y_idx,
                &t.pixels,
                t.clip_rects.iter().map(|r| (r.x, r.y, r.width, r.height)),
                &mut union,
            );
        }

        if let Some(rect) = union {
            // DIAGNOSTIC (revert): muted — scanned the painted union for black
            // pixels and logged [rfx-black]; un-comment to re-enable.
            /*
            // DIAGNOSTIC (revert): does RFX Progressive paint black tiles?
            {
                let iw = u32::from(self.width) as usize;
                let data = self.image.data();
                let (mut total, mut black) = (0usize, 0usize);
                for y in rect.top..rect.bottom {
                    for x in rect.left..rect.right {
                        let i = (y as usize * iw + x as usize) * 4;
                        if i + 2 < data.len() {
                            total += 1;
                            if data[i] == 0 && data[i + 1] == 0 && data[i + 2] == 0 {
                                black += 1;
                            }
                        }
                    }
                }
                if total > 0 && black * 100 / total >= 30 {
                    web_sys::console::warn_1(
                        &format!(
                            "[rfx-black] union=({},{})-({},{}) black={}%",
                            rect.left, rect.top, rect.right, rect.bottom, black * 100 / total
                        )
                        .into(),
                    );
                }
            }
            */
            self.probe_after("rfx_progressive", rect.left, rect.top, rect.right, rect.bottom);
            self.add_dirty_rect(rect);
        }
    }

    /// Composite RFX-Progressive tiles delivered by the Step 2 decode worker as
    /// the self-describing binary blob (see `egfx_decoder`): `u32 tile_count`,
    /// then per tile `{u16 x_idx, u16 y_idx, u16 clip_count, clip_count × u16
    /// x/y/width/height}`, then `tile_count × 16384` RGBA bytes (tile i at
    /// `rgba_start + i*16384`). Parses + blits with the same clip math as
    /// [`Self::blit_rfx_tiles`] (shared `composite_rfx_tile`). A malformed or
    /// truncated blob is dropped (never panics — the blob is trusted but bounds
    /// are validated defensively, and wasm32 `usize` is 32-bit).
    pub(crate) fn blit_rfx_blob(&mut self, surface_origin_x: u32, surface_origin_y: u32, blob: &[u8]) {
        const TILE_PIXELS_LEN: usize = 64 * 64 * 4;
        if blob.len() < 4 {
            return;
        }
        let tile_count = u32::from_le_bytes([blob[0], blob[1], blob[2], blob[3]]) as usize;
        let mut off = 4usize;

        // Parse all per-tile headers (grid position + clip rects); the packed
        // RGBA section follows every header.
        let mut headers: Vec<(u16, u16, Vec<(u16, u16, u16, u16)>)> = Vec::with_capacity(tile_count);
        for _ in 0..tile_count {
            if off + 6 > blob.len() {
                return;
            }
            let x_idx = u16::from_le_bytes([blob[off], blob[off + 1]]);
            let y_idx = u16::from_le_bytes([blob[off + 2], blob[off + 3]]);
            let clip_count = u16::from_le_bytes([blob[off + 4], blob[off + 5]]) as usize;
            off += 6;
            let mut clips = Vec::with_capacity(clip_count);
            for _ in 0..clip_count {
                if off + 8 > blob.len() {
                    return;
                }
                clips.push((
                    u16::from_le_bytes([blob[off], blob[off + 1]]),
                    u16::from_le_bytes([blob[off + 2], blob[off + 3]]),
                    u16::from_le_bytes([blob[off + 4], blob[off + 5]]),
                    u16::from_le_bytes([blob[off + 6], blob[off + 7]]),
                ));
                off += 8;
            }
            headers.push((x_idx, y_idx, clips));
        }

        let rgba_start = off;
        let Some(rgba_bytes) = tile_count.checked_mul(TILE_PIXELS_LEN) else {
            return;
        };
        let Some(rgba_end) = rgba_start.checked_add(rgba_bytes) else {
            return;
        };
        if rgba_end > blob.len() {
            return;
        }

        let img_w = u32::from(self.width);
        let img_h = u32::from(self.height);
        let stride = img_w as usize * 4;
        let buf = self.image.data_mut();
        let mut union: Option<DirtyRect> = None;
        for (i, (x_idx, y_idx, clips)) in headers.iter().enumerate() {
            let pix_start = rgba_start + i * TILE_PIXELS_LEN;
            let pixels = &blob[pix_start..pix_start + TILE_PIXELS_LEN];
            composite_rfx_tile(
                buf,
                stride,
                img_w,
                img_h,
                surface_origin_x,
                surface_origin_y,
                *x_idx,
                *y_idx,
                pixels,
                clips.iter().copied(),
                &mut union,
            );
        }
        if let Some(rect) = union {
            self.probe_after("rfx_progressive", rect.left, rect.top, rect.right, rect.bottom);
            self.add_dirty_rect(rect);
        }
    }

    /// Evict a single `(surface_id, codec_context_id)` progressive decoder
    /// context. The surface's overall decoder stays — Windows may open a new
    /// context on the same surface immediately.
    ///
    /// `codec_context_id == u32::MAX` is the WHOLE-SURFACE sentinel sent when
    /// Windows deletes the surface (see `EgfxDecoder::delete_context` — same
    /// contract for the offload path): drop the surface's decoder entirely so
    /// a recreated surface with a reused id starts from a clean baseline.
    pub fn delete_progressive_context(&mut self, surface_id: u32, codec_context_id: u32) {
        if codec_context_id == u32::MAX {
            self.progressive_decoders.retain(|(id, _)| *id != surface_id);
            return;
        }
        if let Some((_, dec)) = self
            .progressive_decoders
            .iter_mut()
            .find(|(id, _)| *id == surface_id)
        {
            dec.delete_context(codec_context_id);
        }
    }

    /// Fill the listed rectangles on the surface with `(r, g, b, 0xff)`.
    /// EGFX SolidFill PDU semantics — Windows uses this for solid-color
    /// regions (dialog backgrounds, panel fills) that would otherwise be
    /// expensive to send as a full bitmap.
    pub fn apply_solid_fill(
        &mut self,
        _surface_id: u32,
        r: u8,
        g: u8,
        b: u8,
        rects: &[(u32, u32, u32, u32)],
    ) -> Result<()> {
        let img_w = u32::from(self.width);
        let img_h = u32::from(self.height);
        // DIAGNOSTIC (revert): muted — logged [sf] each SolidFill's color + first
        // rect; un-comment to re-enable.
        /*
        // DIAGNOSTIC (revert): log every SolidFill's color + first rect, so we
        // can see whether SolidFill paints the chrome (title bar / taskbar)
        // black. Low volume (a handful per session).
        web_sys::console::warn_1(
            &format!(
                "[sf] color=({r},{g},{b}) n={} first={:?}",
                rects.len(),
                rects.first(),
            )
            .into(),
        );
        */
        let stride = img_w as usize * 4;
        let buf = self.image.data_mut();
        let mut union: Option<DirtyRect> = None;
        for &(left, top, right, bottom) in rects {
            let l = left.min(img_w);
            let t = top.min(img_h);
            let r_ = right.min(img_w);
            let b_ = bottom.min(img_h);
            if r_ <= l || b_ <= t {
                continue;
            }
            for y in t..b_ {
                let row_start = y as usize * stride + l as usize * 4;
                let row_end = y as usize * stride + r_ as usize * 4;
                let row = &mut buf[row_start..row_end];
                for px in row.chunks_exact_mut(4) {
                    px[0] = r;
                    px[1] = g;
                    px[2] = b;
                    px[3] = 0xff;
                }
            }
            let rect = DirtyRect {
                left: l,
                top: t,
                right: r_,
                bottom: b_,
            };
            union = Some(match union {
                Some(u) => u.union(rect),
                None => rect,
            });
        }
        if let Some(u) = union {
            self.probe_after("solid_fill", u.left, u.top, u.right, u.bottom);
            self.add_dirty_rect(u);
        }
        Ok(())
    }

    /// Copy the given rectangle of the surface image into the bitmap cache
    /// at `cache_slot`. Used by Windows so later `CacheToSurface` PDUs can
    /// replay this content at many destination points without re-sending
    /// the pixel data.
    pub fn apply_surface_to_cache(
        &mut self,
        _surface_id: u32,
        cache_slot: u32,
        src_left: u32,
        src_top: u32,
        src_right: u32,
        src_bottom: u32,
    ) -> Result<()> {
        let img_w = u32::from(self.width);
        let img_h = u32::from(self.height);
        let l = src_left.min(img_w);
        let t = src_top.min(img_h);
        let r = src_right.min(img_w);
        let b = src_bottom.min(img_h);
        if r <= l || b <= t {
            // Empty source rect — store an empty entry so later
            // CacheToSurface hits don't panic.
            store_cache_entry(
                &mut self.bitmap_cache,
                cache_slot,
                BitmapCacheEntry {
                    width: 0,
                    height: 0,
                    rgba: Vec::new(),
                },
            );
            return Ok(());
        }
        let w = r - l;
        let h = b - t;
        let stride = img_w as usize * 4;
        let src = self.image.data();
        let mut rgba = Vec::with_capacity(w as usize * h as usize * 4);
        for y in t..b {
            let row_start = y as usize * stride + l as usize * 4;
            let row_end = row_start + w as usize * 4;
            rgba.extend_from_slice(&src[row_start..row_end]);
        }
        // The snapshot value at each PROBE sample point covered by this source
        // rect feeds the (muted) [pixhist] timeline.
        self.probe_after("surface_to_cache", l, t, r, b);
        store_cache_entry(
            &mut self.bitmap_cache,
            cache_slot,
            BitmapCacheEntry {
                width: w,
                height: h,
                rgba,
            },
        );
        Ok(())
    }

    /// Blit cache slot contents onto the surface at each destination
    /// top-left point. The cached rect's width/height is set when the
    /// slot was populated by `apply_surface_to_cache`.
    pub fn apply_cache_to_surface(
        &mut self,
        _surface_id: u32,
        cache_slot: u32,
        dest_points: &[(u32, u32)],
    ) -> Result<()> {
        let entry = match self.bitmap_cache.get(&cache_slot) {
            Some(e) => e,
            None => {
                // DIAGNOSTIC (revert): muted — logged [cold-cache] on a persistent-
                // cache miss AND painted a cyan marker at each cold-cache dest
                // point; un-comment to re-enable.
                /*
                // Cache miss: persistent-cache cold reference — server stamps
                // a slot it expects the client to have from a prior session.
                // We don't store cross-session, so the destination pixels
                // are left as init-black, which propagates as black tiles
                // when later SurfaceToCache snapshots this region. Log so
                // the operator can see how many cold references happen.
                web_sys::console::warn_1(
                    &format!(
                        "[cold-cache] CacheToSurface slot={} n_pts={} first_dest={:?} \
                         — slot not populated this session; skipping (persistent-cache miss)",
                        cache_slot,
                        dest_points.len(),
                        dest_points.first(),
                    )
                    .into(),
                );
                // DIAGNOSTIC (revert): paint a cyan marker at each cold-cache
                // dest point so bitmap-cache cold-misses are visible (vs black).
                let img_w = u32::from(self.width);
                let img_h = u32::from(self.height);
                let stride = img_w as usize * 4;
                let dst = self.image.data_mut();
                for &(px, py) in dest_points {
                    if px >= img_w || py >= img_h {
                        continue;
                    }
                    let mh = 16u32.min(img_h - py);
                    let mw = 16u32.min(img_w - px);
                    for r in 0..mh {
                        let dr = ((py + r) as usize) * stride + (px as usize) * 4;
                        for c in 0..mw as usize {
                            let d = &mut dst[dr + c * 4..dr + c * 4 + 4];
                            d[0] = 0x00;
                            d[1] = 0xFF;
                            d[2] = 0xFF;
                            d[3] = 0xFF;
                        }
                    }
                }
                */
                return Ok(());
            }
        };
        if entry.width == 0 || entry.height == 0 {
            return Ok(());
        }

        let img_w = u32::from(self.width);
        let img_h = u32::from(self.height);
        let stride = img_w as usize * 4;
        let cache_stride = entry.width as usize * 4;
        let (entry_w, entry_h) = (entry.width, entry.height);

        // Blit the cached tile to every destination point.
        {
            let dst = self.image.data_mut();
            for &(px, py) in dest_points {
                if px >= img_w || py >= img_h {
                    continue;
                }
                let copy_w = (img_w - px).min(entry_w);
                let copy_h = (img_h - py).min(entry_h);
                for row in 0..copy_h {
                    let src_off = row as usize * cache_stride;
                    let dst_off = (py as usize + row as usize) * stride + px as usize * 4;
                    let n = copy_w as usize * 4;
                    dst[dst_off..dst_off + n]
                        .copy_from_slice(&entry.rgba[src_off..src_off + n]);
                }
            }
        }
        // Mark each stamp as its own dirty rect rather than one bounding box
        // over all destination points: a CacheToSurface that scatters a tile
        // across the screen — the dominant EGFX op — then uploads only the
        // stamped pixels instead of their enclosing box. `entry`'s borrow ends
        // above, so `add_dirty_rect` can take `&mut self`.
        for &(px, py) in dest_points {
            if px >= img_w || py >= img_h {
                continue;
            }
            let copy_w = (img_w - px).min(entry_w);
            let copy_h = (img_h - py).min(entry_h);
            let rect = DirtyRect {
                left: px,
                top: py,
                right: px + copy_w,
                bottom: py + copy_h,
            };
            self.probe_after("cache_to_surface", rect.left, rect.top, rect.right, rect.bottom);
            self.add_dirty_rect(rect);
        }
        Ok(())
    }

    /// Drop a cache slot. Called when the server signals it no longer
    /// expects the client to retain that entry. Safe if the slot was never
    /// populated.
    pub fn apply_evict_cache_entry(&mut self, cache_slot: u32) {
        self.bitmap_cache.remove(&cache_slot);
    }

    // DIAGNOSTIC (revert): muted — band-scan of the framebuffer counting
    // per-band black%/magenta%/noise, formatted as the [fb-scan] log; the only
    // caller (the scan branch in bump_traffic) is muted too. Un-comment both to
    // re-enable.
    /*
    /// DIAGNOSTIC (revert): scan the framebuffer image in horizontal bands,
    /// reporting per-band pure-black% and a "noise" metric (mean absolute
    /// difference between horizontally-adjacent pixels). This isolates the
    /// "deep-fried" symptom: garbled noise present IN `self.image` means a
    /// decode/cache problem (NOT GL); a band that's clean in content but
    /// shows noise on screen would indict the GL upload/draw path. Flat UI
    /// bands read low noise; garbage reads high. Read-only; O(w*h) — the
    /// caller throttles it.
    pub fn fb_scan_report(&self) -> String {
        let w = u32::from(self.width) as usize;
        let h = u32::from(self.height) as usize;
        let data = self.image.data();
        let bands = 8usize;
        let mut out = format!("[fb-scan] {w}x{h}");
        for band in 0..bands {
            let y0 = band * h / bands;
            let y1 = ((band + 1) * h / bands).max(y0 + 1).min(h);
            let mut black = 0usize;
            let mut magenta = 0usize;
            let mut noise: u64 = 0;
            let mut diffs: u64 = 0;
            for y in y0..y1 {
                let row = y * w * 4;
                for x in 0..w {
                    let i = row + x * 4;
                    if data[i] == 0 && data[i + 1] == 0 && data[i + 2] == 0 {
                        black += 1;
                    } else if data[i] == 0xff && data[i + 1] == 0 && data[i + 2] == 0xff {
                        // DIAGNOSTIC (revert): pure-magenta == never-painted init fill.
                        magenta += 1;
                    }
                    if x + 1 < w {
                        let j = i + 4;
                        noise += i32::abs(data[i] as i32 - data[j] as i32) as u64
                            + i32::abs(data[i + 1] as i32 - data[j + 1] as i32) as u64
                            + i32::abs(data[i + 2] as i32 - data[j + 2] as i32) as u64;
                        diffs += 1;
                    }
                }
            }
            let bandpx = (y1 - y0) * w;
            let blk = if bandpx > 0 { black * 100 / bandpx } else { 0 };
            let mag = if bandpx > 0 { magenta * 100 / bandpx } else { 0 };
            let avg = if diffs > 0 { noise / diffs } else { 0 };
            out.push_str(&format!(" b{band}(y{y0}-{y1} blk={blk}% mag={mag}% noise={avg})"));
        }
        out
    }
    */

    /// Copy a region of the framebuffer image to each destination point.
    /// Source and destination are the same image — Windows uses this to
    /// scroll/move taskbar items, animate hovers, etc. Source pixels are
    /// snapshotted first into a temp buffer so overlapping src/dst regions
    /// (the common case) don't corrupt the copy mid-flight.
    pub fn apply_surface_to_surface(
        &mut self,
        _src_surface_id: u32,
        _dst_surface_id: u32,
        src_left: u32,
        src_top: u32,
        src_right: u32,
        src_bottom: u32,
        dest_points: &[(u32, u32)],
    ) -> Result<()> {
        let img_w = u32::from(self.width);
        let img_h = u32::from(self.height);
        let l = src_left.min(img_w);
        let t = src_top.min(img_h);
        let r = src_right.min(img_w);
        let b = src_bottom.min(img_h);
        if r <= l || b <= t || dest_points.is_empty() {
            return Ok(());
        }
        let w = r - l;
        let h = b - t;
        let stride = img_w as usize * 4;

        // Diagnostic: sample 4 corners + center of the SOURCE region BEFORE
        // the copy. If any corner is black but dst was supposed to receive
        // real content (e.g., the composed selection-over-icon), this points
        // to a SurfaceToSurface reading init-black source pixels.
        // Reuse a pooled buffer for the source snapshot (transient — copied out
        // before the in-place dest blits, since src/dst alias `image`).
        let mut snapshot = core::mem::take(&mut self.s2s_scratch);
        snapshot.clear();
        let src_bytes = self.image.data();
        let sample = |x: u32, y: u32| -> (u8, u8, u8, u8) {
            let off = y as usize * stride + x as usize * 4;
            if off + 3 < src_bytes.len() {
                (src_bytes[off], src_bytes[off + 1], src_bytes[off + 2], src_bytes[off + 3])
            } else {
                (0, 0, 0, 0)
            }
        };
        let _ = sample;
        snapshot.reserve(w as usize * h as usize * 4);
        for y in t..b {
            let row_start = y as usize * stride + l as usize * 4;
            let row_end = row_start + w as usize * 4;
            snapshot.extend_from_slice(&src_bytes[row_start..row_end]);
        }

        let dst = self.image.data_mut();
        let snap_stride = w as usize * 4;
        let mut union: Option<DirtyRect> = None;
        for &(px, py) in dest_points {
            if px >= img_w || py >= img_h {
                continue;
            }
            let copy_w = (img_w - px).min(w);
            let copy_h = (img_h - py).min(h);
            for row in 0..copy_h {
                let src_off = row as usize * snap_stride;
                let dst_off = (py as usize + row as usize) * stride + px as usize * 4;
                let n = copy_w as usize * 4;
                dst[dst_off..dst_off + n]
                    .copy_from_slice(&snapshot[src_off..src_off + n]);
            }
            let rect = DirtyRect {
                left: px,
                top: py,
                right: px + copy_w,
                bottom: py + copy_h,
            };
            union = Some(match union {
                Some(u) => u.union(rect),
                None => rect,
            });
        }
        if let Some(u) = union {
            self.probe_after("surface_to_surface", u.left, u.top, u.right, u.bottom);
            self.add_dirty_rect(u);
        }
        // Return the buffer to the pool for the next SurfaceToSurface.
        self.s2s_scratch = snapshot;
        Ok(())
    }

    pub fn apply_fast_path(&mut self, pdu: &[u8]) -> Result<ProcessOutput> {
        let mut output = WriteBuf::new();
        let class = classify_pdu(pdu);

        // Peek the fast-path update header to log update code + fragmentation
        // flag before processing — even when no Region update results, we
        // want to know what code was being assembled. See [MS-RDPBCGR]
        // 2.2.9.1.2.1 for the TS_FP_UPDATE byte layout: low 4 bits =
        // updateCode, next 2 = fragmentation, top 2 = compression.
        let peek = peek_fast_path_update_byte(pdu);

        let process_start = perf::performance_now();
        let updates = self
            .fast_path
            .process(&mut self.image, pdu, &mut output)
            .map_err(|e| anyhow!("fastpath process failed: {e:?}"))?;
        let elapsed = perf::performance_now() - process_start;
        perf::record_process(elapsed, class);
        perf::maybe_report_slow_pdu(elapsed, class, pdu.len());

        let mut cursors = Vec::new();
        for update in updates {
            match update {
                UpdateKind::Region(rect) => {
                    self.expand_dirty(&rect);
                }
                UpdateKind::PointerBitmap(p) => cursors.push(CursorEvent::Bitmap {
                    rgba: p.bitmap_data.to_vec(),
                    width: p.width,
                    height: p.height,
                    hotspot_x: p.hotspot_x as i32,
                    hotspot_y: p.hotspot_y as i32,
                }),
                UpdateKind::PointerHidden => cursors.push(CursorEvent::Hidden),
                UpdateKind::PointerDefault => cursors.push(CursorEvent::Default),
                UpdateKind::PointerPosition { .. } | UpdateKind::None => {}
            }
        }
        let _ = peek;
        Ok(ProcessOutput {
            response: output.into_inner(),
            cursors,
        })
    }

    /// Accumulate one dirty rectangle for the next render. Merges into an
    /// overlapping tracked rect when possible; otherwise appends, up to
    /// `MAX_DIRTY_RECTS`. On overflow the new rect is unioned into the tracked
    /// rect whose area grows the least, so the set always *covers* every
    /// rectangle ever added (no region is dropped) while staying bounded.
    fn add_dirty_rect(&mut self, rect: DirtyRect) {
        if rect.right <= rect.left || rect.bottom <= rect.top {
            return;
        }
        for d in self.dirty.iter_mut() {
            if d.overlaps(rect) {
                *d = d.union(rect);
                return;
            }
        }
        if self.dirty.len() < MAX_DIRTY_RECTS {
            self.dirty.push(rect);
            return;
        }
        let mut best = 0usize;
        let mut best_growth = u64::MAX;
        for (i, d) in self.dirty.iter().enumerate() {
            let growth = d.union(rect).area().saturating_sub(d.area());
            if growth < best_growth {
                best_growth = growth;
                best = i;
            }
        }
        self.dirty[best] = self.dirty[best].union(rect);
    }

    /// Replace the dirty set with the whole image. Used when a canvas is
    /// (re)registered or the desktop is resized and the next render must
    /// repaint every pixel.
    fn mark_all_dirty(&mut self) {
        self.dirty.clear();
        self.dirty.push(DirtyRect {
            left: 0,
            top: 0,
            right: u32::from(self.width),
            bottom: u32::from(self.height),
        });
    }

    /// Mark the framebuffer as wanting a render without doing GL work.
    /// `flush_render` later turns the accumulated `dirty` rects into one
    /// `render` (one `upload_rect` per disjoint rect + a single draw). EGFX
    /// handlers call this in place of `render` so a wire burst (start_frame …
    /// many tiles … end_frame) becomes one GL flush instead of one per PDU.
    pub fn mark_dirty_pending(&mut self) {
        self.dirty_pending = true;
    }

    /// True iff a coalesced flush is owed.
    pub fn dirty_pending(&self) -> bool {
        self.dirty_pending
    }

    /// Render iff `mark_dirty_pending` was called since the last flush.
    /// Clears the pending flag unconditionally; `render` itself also
    /// drains `self.dirty`, so back-to-back flushes are no-ops.
    pub fn flush_render(&mut self) -> Result<()> {
        if !self.dirty_pending {
            return Ok(());
        }
        self.dirty_pending = false;
        self.render()
    }

    /// Current surface dimensions `(width, height)` in pixels. Used by the Step 2
    /// offload path to tell the decode worker the RFX tile-grid size.
    pub(crate) fn dims(&self) -> (u16, u16) {
        (self.width, self.height)
    }

    pub fn render(&mut self) -> Result<()> {
        if self.width == 0 || self.height == 0 {
            return Ok(());
        }
        let dirty = std::mem::take(&mut self.dirty);
        let image_width = u32::from(self.width);
        let src = self.image.data();

        // DIAGNOSTIC (revert): muted — logged each sample point's presented RGBA
        // on flush; un-comment to re-enable.
        /*
        // DIAGNOSTIC (revert): log what each sample point is PRESENTED as on this
        // flush (only if it's in the uploaded dirty band). If a point shows
        // black -> grey -> black across flushes, we're presenting half-applied
        // frames (no EndFrame gating) = the flicker.
        if let Some(d) = dirty.as_ref() {
            for &(px, py) in PROBES {
                if px >= d.left && px < d.right && py >= d.top && py < d.bottom {
                    let i = (py as usize * image_width as usize + px as usize) * 4;
                    if i + 3 < src.len() {
                        web_sys::console::warn_1(
                            &format!(
                                "[present] ({px},{py}) -> ({},{},{},{})",
                                src[i], src[i + 1], src[i + 2], src[i + 3]
                            )
                            .into(),
                        );
                    }
                }
            }
        }
        */

        for (_, view) in self.canvases.iter() {
            let vp = view.viewport;
            let mut uploaded = false;
            // Upload each disjoint dirty rect intersected with this canvas's
            // viewport (in BOTH axes, image coords) so we transfer only the
            // changed pixels — not a single near-fullscreen bounding box when
            // scattered EGFX stamps land in the same frame.
            for d in &dirty {
                let l = d.left.max(vp.x);
                let t = d.top.max(vp.y);
                let r = d.right.min(vp.x + vp.width);
                let b = d.bottom.min(vp.y + vp.height);
                if l >= r || t >= b {
                    continue;
                }
                // dst = texture (viewport-local) offset; src = framebuffer offset.
                let (dst_x, dst_y, w, h) = (l - vp.x, t - vp.y, r - l, b - t);
                let put_start = perf::performance_now();
                view.painter.upload_rect(
                    src,
                    image_width,
                    dst_x,
                    dst_y,
                    dst_x + vp.x, // src_x in framebuffer
                    dst_y + vp.y, // src_y in framebuffer
                    w,
                    h,
                )?;
                perf::record_put_image(perf::performance_now() - put_start, w, h);
                uploaded = true;
            }
            // One quad draw after all rects are uploaded (not one per rect). A
            // canvas that uploaded nothing still draws its first frame so it's
            // not left blank; afterwards it keeps its image via
            // preserveDrawingBuffer and needn't redraw.
            if uploaded || !view.painter.presented() {
                view.painter.draw();
            }
        }
        Ok(())
    }

    fn expand_dirty(&mut self, rect: &InclusiveRectangle) {
        let r = DirtyRect {
            left: rect.left as u32,
            top: rect.top as u32,
            right: (rect.right as u32).saturating_add(1).min(self.width as u32),
            bottom: (rect.bottom as u32).saturating_add(1).min(self.height as u32),
        };
        self.add_dirty_rect(r);
    }
}

/// Composite one decoded 64×64 RFX-Progressive tile's RGBA `pixels` into the
/// framebuffer `buf` (row stride `stride`, image `img_w`×`img_h`), clipped to
/// `clip_rects` (each `(x, y, width, height)` in surface-relative coords) and to
/// the tile's grid cell at `(x_idx, y_idx)` offset by the surface origin. Each
/// painted rectangle is unioned into `union`. FreeRDP `update_tiles` semantics:
/// only the damaged region is repainted; prior-frame content is preserved
/// elsewhere. Shared by the inline [`Framebuffer::blit_rfx_tiles`] (DecodedTile
/// input) and the decode-worker [`Framebuffer::blit_rfx_blob`] (parsed-blob)
/// paths so the clip math has a single definition.
#[allow(clippy::too_many_arguments)]
fn composite_rfx_tile(
    buf: &mut [u8],
    stride: usize,
    img_w: u32,
    img_h: u32,
    surface_origin_x: u32,
    surface_origin_y: u32,
    x_idx: u16,
    y_idx: u16,
    pixels: &[u8],
    clip_rects: impl IntoIterator<Item = (u16, u16, u16, u16)>,
    union: &mut Option<DirtyRect>,
) {
    let tile_w: u32 = 64;
    let tile_h: u32 = 64;
    let dx = surface_origin_x.saturating_add(u32::from(x_idx) * tile_w);
    let dy = surface_origin_y.saturating_add(u32::from(y_idx) * tile_h);
    if dx >= img_w || dy >= img_h {
        return;
    }
    let tile_x1 = (dx + tile_w).min(img_w);
    let tile_y1 = (dy + tile_h).min(img_h);
    let src_stride = tile_w as usize * 4;

    for (rx, ry, rw, rh) in clip_rects {
        let cx0 = surface_origin_x.saturating_add(u32::from(rx));
        let cy0 = surface_origin_y.saturating_add(u32::from(ry));
        let ix0 = dx.max(cx0);
        let iy0 = dy.max(cy0);
        let ix1 = tile_x1.min(cx0.saturating_add(u32::from(rw)));
        let iy1 = tile_y1.min(cy0.saturating_add(u32::from(rh)));
        if ix0 >= ix1 || iy0 >= iy1 {
            continue;
        }
        let copy_bytes = (ix1 - ix0) as usize * 4;
        for y in iy0..iy1 {
            let src_off = (y - dy) as usize * src_stride + (ix0 - dx) as usize * 4;
            let dst_off = y as usize * stride + ix0 as usize * 4;
            buf[dst_off..dst_off + copy_bytes].copy_from_slice(&pixels[src_off..src_off + copy_bytes]);
        }
        let rect = DirtyRect { left: ix0, top: iy0, right: ix1, bottom: iy1 };
        *union = Some(match *union {
            Some(d) => d.union(rect),
            None => rect,
        });
    }
}

/// Blit a `src_w`x`src_h` source in **BGRA** into the RGBA `image`
/// (`img_w`x`img_h`) at desktop coords (`dest_x`, `dest_y`), swapping
/// B<->R and forcing alpha opaque. Clips on all four edges (negative
/// origin clips the source's leading rows/cols). Fully off-image is a
/// no-op. Never panics, never bails — mirrors `blit_rgba`'s clip policy.
fn blit_bgra_clipped(
    image: &mut [u8],
    img_w: u32,
    img_h: u32,
    dest_x: i32,
    dest_y: i32,
    src_w: u32,
    src_h: u32,
    bgra: &[u8],
) {
    if src_w == 0 || src_h == 0 {
        return;
    }
    let skip_cols = if dest_x < 0 { (-dest_x) as u32 } else { 0 };
    let skip_rows = if dest_y < 0 { (-dest_y) as u32 } else { 0 };
    if skip_cols >= src_w || skip_rows >= src_h {
        return;
    }
    let dst_x0 = dest_x.max(0) as u32;
    let dst_y0 = dest_y.max(0) as u32;
    if dst_x0 >= img_w || dst_y0 >= img_h {
        return;
    }
    let copy_w = (src_w - skip_cols).min(img_w - dst_x0);
    let copy_h = (src_h - skip_rows).min(img_h - dst_y0);
    let row_bytes = copy_w as usize * 4;
    for row in 0..copy_h {
        let src_row = skip_rows + row;
        let dst_row = dst_y0 + row;
        let src_base = ((src_row * src_w + skip_cols) * 4) as usize;
        let dst_base = ((dst_row * img_w + dst_x0) * 4) as usize;
        // Slice each row once so the per-pixel BGRA→RGBA swap is bounds-check-
        // free (one slice check per row, not per index).
        let src_px = &bgra[src_base..src_base + row_bytes];
        let dst_px = &mut image[dst_base..dst_base + row_bytes];
        for (s_px, d_px) in src_px.chunks_exact(4).zip(dst_px.chunks_exact_mut(4)) {
            // ClearCodec is prior-frame-relative: a partial-coverage tile only
            // paints some pixels, leaving the rest implicitly equal to the
            // previous frame ([MS-RDPEGFX] 2.2.4.2 — e.g. V-bar columns over a
            // stable acrylic-chrome background, or a sub-tile subcodec region).
            // The upstream decoder zero-inits its buffer and sets alpha = 0xFF
            // on every pixel it actually writes, so a source alpha of 0 marks
            // an UNCOVERED pixel that must PRESERVE the existing framebuffer
            // content — not be stamped opaque black. (This branch blocks
            // autovectorization, but the row-slice form is materially tighter.)
            if s_px[3] == 0 {
                continue;
            }
            d_px[0] = s_px[2]; // R
            d_px[1] = s_px[1]; // G
            d_px[2] = s_px[0]; // B
            d_px[3] = 0xff; // opaque
        }
    }
}

/// Peek the inner `TS_FP_UPDATE_HEADER` byte after skipping the outer
/// `FastPathHeader` (1-byte flags + 1- or 2-byte PER length). Returns a
/// human-readable string for diagnostic logs. Inner header layout per
/// [MS-RDPBCGR] 2.2.9.1.2.1.1: low 4 bits = updateCode, bits 4-5 =
/// fragmentation, bits 6-7 = compression.
fn peek_fast_path_update_byte(pdu: &[u8]) -> String {
    if pdu.len() < 2 {
        return format!("<too short: {} bytes>", pdu.len());
    }
    // PER aligned length: if pdu[1] high bit is 1, length is two bytes (high
    // 7 bits of pdu[1] + pdu[2]); otherwise one byte (low 7 bits of pdu[1]).
    let outer_len = if pdu[1] & 0x80 != 0 { 3 } else { 2 };
    let Some(&inner) = pdu.get(outer_len) else {
        return format!(
            "<no inner header: outer_len={outer_len} pdu_len={}>",
            pdu.len()
        );
    };
    let code = inner & 0x0f;
    let frag = (inner >> 4) & 0x03;
    let comp = (inner >> 6) & 0x03;
    let code_name = match code {
        0x0 => "Orders",
        0x1 => "Bitmap",
        0x2 => "Palette",
        0x3 => "Synchronize",
        0x4 => "SurfaceCommands",
        0x5 => "PointerHidden",
        0x6 => "PointerDefault",
        0x7 => "PointerPosition",
        0x8 => "ColorPointer",
        0x9 => "CachedPointer",
        0xa => "NewPointer",
        0xb => "LargePointer",
        _ => "?",
    };
    let frag_name = match frag {
        0 => "single",
        1 => "last",
        2 => "first",
        3 => "next",
        _ => "?",
    };
    format!(
        "outer_len={outer_len} inner=0x{inner:02x} code={code_name}({code}) frag={frag_name}({frag}) comp={comp}"
    )
}

fn store_cache_entry(
    cache: &mut HashMap<u32, BitmapCacheEntry>,
    slot: u32,
    entry: BitmapCacheEntry,
) {
    cache.insert(slot, entry);
}

/// Draw a 1-pixel-wide RGBA magenta border around the rectangle
/// `(px, py)` size `w` × `h` directly into the framebuffer image. Used
/// only as a diagnostic so the operator can visually identify tainted
/// `CacheToSurface` replays on screen. `stride` is the row stride in
/// bytes; `img_w` / `img_h` clip the rect to image bounds.
fn paint_magenta_border(
    buf: &mut [u8],
    stride: usize,
    img_w: u32,
    img_h: u32,
    px: u32,
    py: u32,
    w: u32,
    h: u32,
) {
    if w == 0 || h == 0 {
        return;
    }
    let x0 = px.min(img_w);
    let y0 = py.min(img_h);
    let x1 = (px + w).min(img_w);
    let y1 = (py + h).min(img_h);
    if x1 <= x0 || y1 <= y0 {
        return;
    }
    let put = |buf: &mut [u8], x: u32, y: u32| {
        let off = y as usize * stride + x as usize * 4;
        if off + 3 < buf.len() {
            buf[off] = 0xff;
            buf[off + 1] = 0x00;
            buf[off + 2] = 0xff;
            buf[off + 3] = 0xff;
        }
    };
    // Top + bottom rows.
    for x in x0..x1 {
        put(buf, x, y0);
        if y1 > 0 {
            put(buf, x, y1 - 1);
        }
    }
    // Left + right columns (skip the four corners already drawn).
    for y in y0..y1 {
        put(buf, x0, y);
        if x1 > 0 {
            put(buf, x1 - 1, y);
        }
    }
}

fn lookup_or_create_progressive<'a>(
    decoders: &'a mut Vec<(u32, ProgressiveDecoder)>,
    surface_id: u32,
) -> &'a mut ProgressiveDecoder {
    if let Some(idx) = decoders.iter().position(|(id, _)| *id == surface_id) {
        &mut decoders[idx].1
    } else {
        decoders.push((surface_id, ProgressiveDecoder::new()));
        &mut decoders.last_mut().unwrap().1
    }
}

pub fn require_nonzero_dims(width: u16, height: u16) -> Result<()> {
    if width == 0 || height == 0 {
        bail!("invalid screen dimensions: {width}x{height}");
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn rect(left: u32, top: u32, right: u32, bottom: u32) -> DirtyRect {
        DirtyRect { left, top, right, bottom }
    }

    /// Disjoint stamps stay as separate rects — the whole point of the multi-
    /// rect dirty set (so `render` uploads only the changed pixels, not the
    /// bounding box that spans the gap between them).
    #[test]
    fn add_dirty_rect_keeps_disjoint_rects_separate() {
        let mut fb = Framebuffer::new(1003, 1004, 256, 256).unwrap();
        fb.dirty.clear();
        fb.add_dirty_rect(rect(0, 0, 16, 16));
        fb.add_dirty_rect(rect(100, 100, 120, 120));
        assert_eq!(fb.dirty.len(), 2, "disjoint rects must not be merged");
    }

    #[test]
    fn add_dirty_rect_merges_overlapping() {
        let mut fb = Framebuffer::new(1003, 1004, 256, 256).unwrap();
        fb.dirty.clear();
        fb.add_dirty_rect(rect(0, 0, 20, 20));
        fb.add_dirty_rect(rect(10, 10, 30, 30));
        assert_eq!(fb.dirty.len(), 1, "overlapping rects merge into one");
        let d = fb.dirty[0];
        assert_eq!((d.left, d.top, d.right, d.bottom), (0, 0, 30, 30));
    }

    /// Edge-touching (adjacent, non-overlapping) rects are NOT merged: the
    /// half-open overlap test must treat a shared edge as no overlap.
    #[test]
    fn add_dirty_rect_does_not_merge_adjacent() {
        let mut fb = Framebuffer::new(1003, 1004, 256, 256).unwrap();
        fb.dirty.clear();
        fb.add_dirty_rect(rect(0, 0, 10, 10));
        fb.add_dirty_rect(rect(10, 0, 20, 10)); // shares the x=10 edge
        assert_eq!(fb.dirty.len(), 2);
    }

    #[test]
    fn add_dirty_rect_ignores_empty() {
        let mut fb = Framebuffer::new(1003, 1004, 256, 256).unwrap();
        fb.dirty.clear();
        fb.add_dirty_rect(rect(10, 10, 10, 20)); // zero width
        fb.add_dirty_rect(rect(10, 10, 20, 10)); // zero height
        assert!(fb.dirty.is_empty());
    }

    /// Past the cap the set stays bounded AND still covers every rect ever
    /// added — overflow may over-upload (merged bounding boxes) but must never
    /// drop a region, or the screen would show stale pixels there.
    #[test]
    fn add_dirty_rect_overflow_stays_bounded_and_covers_all() {
        let mut fb = Framebuffer::new(1003, 1004, 2048, 2048).unwrap();
        fb.dirty.clear();
        let mut added = Vec::new();
        for i in 0..(MAX_DIRTY_RECTS as u32 + 20) {
            let x = (i % 8) * 200;
            let y = (i / 8) * 200;
            let r = rect(x, y, x + 8, y + 8); // disjoint 8x8 stamps on a 200px grid
            fb.add_dirty_rect(r);
            added.push(r);
        }
        assert!(fb.dirty.len() <= MAX_DIRTY_RECTS, "dirty set must stay bounded");
        for a in &added {
            let covered = fb.dirty.iter().any(|d| {
                d.left <= a.left && d.top <= a.top && d.right >= a.right && d.bottom >= a.bottom
            });
            assert!(covered, "rect {a:?} not covered by tracked set {:?}", fb.dirty);
        }
    }

    #[test]
    fn mark_all_dirty_resets_to_full_screen() {
        let mut fb = Framebuffer::new(1003, 1004, 128, 64).unwrap();
        fb.add_dirty_rect(rect(0, 0, 8, 8));
        fb.mark_all_dirty();
        assert_eq!(fb.dirty.len(), 1);
        let d = fb.dirty[0];
        assert_eq!((d.left, d.top, d.right, d.bottom), (0, 0, 128, 64));
    }

    #[test]
    fn blit_bgra_clipped_swaps_and_clips() {
        let (iw, ih) = (4u32, 2u32);
        let mut img = vec![0u8; (iw * ih * 4) as usize];
        for px in img.chunks_exact_mut(4) {
            px[3] = 0xff;
        }
        let src: Vec<u8> = std::iter::repeat([10u8, 20, 30, 40])
            .take(4)
            .flat_map(|a| a.into_iter())
            .collect();
        blit_bgra_clipped(&mut img, iw, ih, 3, 0, 2, 2, &src);
        let off = ((0 * iw + 3) * 4) as usize;
        assert_eq!(&img[off..off + 4], &[30, 20, 10, 0xff]);
        let off2 = ((1 * iw + 3) * 4) as usize;
        assert_eq!(&img[off2..off2 + 4], &[30, 20, 10, 0xff]);
        assert_eq!(&img[0..4], &[0, 0, 0, 0xff]);
    }

    /// ClearCodec is prior-frame-relative: a partial-coverage tile leaves
    /// uncovered pixels at `alpha == 0` in the decode buffer (every covered
    /// write sets `alpha = 0xFF`). The blit must PRESERVE the existing
    /// framebuffer there, not stamp it opaque black — this is the Win11
    /// acrylic-chrome black-rect bug.
    #[test]
    fn blit_bgra_clipped_preserves_dest_where_src_alpha_zero() {
        let (iw, ih) = (2u32, 1u32);
        // Prior framebuffer content: solid red (RGBA).
        let mut img = vec![0u8; (iw * ih * 4) as usize];
        for px in img.chunks_exact_mut(4) {
            px[0] = 0xff; // R
            px[1] = 0x00;
            px[2] = 0x00;
            px[3] = 0xff;
        }
        // Source BGRA: col0 covered (alpha 0xFF, BGRA blue), col1 uncovered
        // (alpha 0x00 — should preserve the prior red).
        let src: Vec<u8> = vec![
            0xff, 0x00, 0x00, 0xff, // col0: covered, BGRA blue (B=0xff)
            0x00, 0x00, 0x00, 0x00, // col1: uncovered (alpha 0)
        ];
        blit_bgra_clipped(&mut img, iw, ih, 0, 0, 2, 1, &src);
        // col0: covered → RGBA blue (B↔R swap).
        assert_eq!(
            &img[0..4],
            &[0x00, 0x00, 0xFF, 0xFF],
            "covered pixel (alpha 0xFF) should be painted"
        );
        // col1: uncovered (src alpha 0) → prior red preserved, NOT black.
        assert_eq!(
            &img[4..8],
            &[0xFF, 0x00, 0x00, 0xFF],
            "uncovered pixel (src alpha 0) must preserve prior content, not be stamped black"
        );
    }

    /// Roundtrip test: encode a 4×4 solid blue tile with `ClearCodecEncoder`,
    /// decode it via `apply_clearcodec` (which uses the upstream
    /// `ironrdp_graphics::clearcodec::ClearCodecDecoder`), and assert the
    /// framebuffer pixels are painted the expected blue (B=0xFF, G=0, R=0).
    ///
    /// The upstream encoder produces a PDU that the upstream decoder accepts,
    /// so this test validates the full encode→decode→blit pipeline without
    /// depending on a captured PDU whose format might not pass upstream
    /// strict-validation rules.
    #[test]
    fn clearcodec_roundtrip_paints_nonblack() {
        let (w, h): (u32, u32) = (4, 4);
        // Solid blue in BGRA (B=0xFF, G=0, R=0, A=0xFF).
        let bgra: Vec<u8> = (0..(w * h) as usize)
            .flat_map(|_| [0xFFu8, 0x00, 0x00, 0xFF])
            .collect();
        let mut enc = ironrdp_graphics::clearcodec::ClearCodecEncoder::new();
        let pdu = enc.encode(&bgra, w as u16, h as u16);
        // Construct a Framebuffer large enough for the 4×4 dest rect.
        // io_channel_id / user_channel_id are arbitrary; we never call feed_bytes.
        let mut fb = Framebuffer::new(1003, 1004, 64, 64).unwrap();
        fb.apply_clearcodec(1, 0, 0, w, h, &pdu).unwrap();
        let img = fb.image.data();
        // After BGRA→RGBA swap: B=0xFF stays at position 2, R=0 stays at position 0.
        // BGRA [0xFF, 0x00, 0x00, 0xFF] → RGBA [0x00, 0x00, 0xFF, 0xFF] (pure blue).
        //
        // The framebuffer is 64 wide, so the 4×4 painted rect occupies:
        //   row r (0..4), columns 0..4 → byte offset (r * 64 + col) * 4.
        // We must check those specific pixels rather than the first 16 flat entries.
        let fb_w = 64usize;
        for row in 0..h as usize {
            for col in 0..w as usize {
                let off = (row * fb_w + col) * 4;
                let pixel = &img[off..off + 4];
                assert_eq!(
                    pixel,
                    &[0x00, 0x00, 0xFF, 0xFF],
                    "expected pure-blue RGBA at ({col},{row}), got {pixel:?}"
                );
            }
        }
        // Sanity: a pixel outside the painted rect should still be the opaque
        // black init fill (`new_init_image`).
        let outside_off = (0 * fb_w + w as usize) * 4; // (row=0, col=w)
        assert_eq!(
            &img[outside_off..outside_off + 4],
            &[0x00, 0x00, 0x00, 0xFF],
            "pixel outside painted rect should be the opaque-black init fill"
        );
    }

    #[test]
    fn blit_bgra_clipped_negative_origin_and_fully_offscreen() {
        let (iw, ih) = (4u32, 2u32);
        let mut img = vec![0u8; (iw * ih * 4) as usize];
        for px in img.chunks_exact_mut(4) {
            px[3] = 0xff;
        }
        let src: Vec<u8> = std::iter::repeat([1u8, 2, 3, 4])
            .take(8)
            .flat_map(|a| a.into_iter())
            .collect();
        blit_bgra_clipped(&mut img, iw, ih, -2, 0, 4, 2, &src);
        assert_eq!(&img[0..4], &[3, 2, 1, 0xff]);
        let mut img2 = vec![0u8; (iw * ih * 4) as usize];
        for px in img2.chunks_exact_mut(4) {
            px[3] = 0xff;
        }
        blit_bgra_clipped(&mut img2, iw, ih, 100, 100, 4, 2, &src);
        assert!(img2.chunks_exact(4).all(|p| p == [0, 0, 0, 0xff]));
    }

    /// Paint a known 8×8 red patch, cache it via `apply_surface_to_cache`,
    /// then stamp it at a new location via `apply_cache_to_surface`.
    /// The stamped pixels must be pixel-exact reproductions of the source.
    #[test]
    fn cache_roundtrip_reproduces_pixels() {
        let mut fb = Framebuffer::new(1003, 1004, 64, 64).unwrap();
        // Paint an 8×8 red patch at (0, 0).
        let patch: Vec<u8> = std::iter::repeat([200u8, 10, 10, 0xff])
            .take(64)
            .flat_map(|a| a.into_iter())
            .collect();
        fb.blit_rgba(0, 0, 8, 8, &patch).unwrap();
        // Cache that rect into slot 5.
        fb.apply_surface_to_cache(1, 5, 0, 0, 8, 8).unwrap();
        // Stamp it at (16, 16).
        fb.apply_cache_to_surface(1, 5, &[(16, 16)]).unwrap();
        let img = fb.image.data();
        let off = ((16 * 64 + 16) * 4) as usize;
        assert_eq!(
            &img[off..off + 4],
            &[200, 10, 10, 0xff],
            "stamped pixel at (16,16) should match cached red patch"
        );
        // Also verify the full 8×8 stamped region is correct.
        for row in 0..8usize {
            for col in 0..8usize {
                let o = ((16 + row) * 64 + (16 + col)) * 4;
                assert_eq!(
                    &img[o..o + 4],
                    &[200, 10, 10, 0xff],
                    "stamped pixel at ({},{}) should be red", 16 + col, 16 + row
                );
            }
        }
    }

    /// Stamp a cached 8×8 rect at a dest point near the edge so it overflows
    /// the image boundary — must NOT panic or return Err; pixels within the
    /// image are painted, pixels outside are silently clipped.
    #[test]
    fn cache_to_surface_offscreen_dest_is_clipped() {
        let mut fb = Framebuffer::new(1003, 1004, 64, 64).unwrap();
        // Paint and cache a known 8×8 patch.
        let patch: Vec<u8> = std::iter::repeat([1u8, 2, 3, 0xff])
            .take(64)
            .flat_map(|a| a.into_iter())
            .collect();
        fb.blit_rgba(0, 0, 8, 8, &patch).unwrap();
        fb.apply_surface_to_cache(1, 7, 0, 0, 8, 8).unwrap();
        // dest (60, 60) means cols 60..68 and rows 60..68 — last 4 cols/rows
        // fall outside the 64×64 image; must NOT panic or bail.
        fb.apply_cache_to_surface(1, 7, &[(60, 60)]).unwrap();
        // Verify the in-bounds portion was painted.
        let img = fb.image.data();
        let off = ((60 * 64 + 60) * 4) as usize;
        assert_eq!(
            &img[off..off + 4],
            &[1, 2, 3, 0xff],
            "in-bounds corner pixel at (60,60) should have been painted"
        );
    }

    /// A mid-session DisplayControl resize (e.g. a monitor added to the right)
    /// grows the framebuffer but the server only sends incremental updates
    /// afterward — it does NOT repaint the existing desktop. So `resize_*` must
    /// carry the already-painted pixels into the larger buffer, not blank them.
    #[test]
    fn resize_grow_preserves_painted_content() {
        let mut fb = Framebuffer::new(1003, 1004, 100, 50).unwrap();
        let red: Vec<u8> = std::iter::repeat([255u8, 0, 0, 0xff])
            .take(16)
            .flat_map(|a| a.into_iter())
            .collect();
        fb.blit_rgba(10, 10, 4, 4, &red).unwrap();

        fb.resize_preserving_canvases(200, 80).unwrap();

        let w = 200usize;
        let off = (10 * w + 10) * 4;
        let d = fb.image.data();
        assert_eq!(
            &d[off..off + 4],
            &[255, 0, 0, 0xff],
            "painted content must survive a grow resize, not be blanked to black"
        );
    }

    /// The EGFX bitmap cache is connection-scoped and survives a ResetGraphics
    /// layout change (the server evicts entries only via EvictCacheEntry). After
    /// a resize the server replays `CacheToSurface` against slots populated
    /// before it — clearing the cache turns those replays into black cold-misses.
    #[test]
    fn resize_preserves_bitmap_cache() {
        let mut fb = Framebuffer::new(1003, 1004, 64, 64).unwrap();
        let patch: Vec<u8> = std::iter::repeat([1u8, 2, 3, 0xff])
            .take(64)
            .flat_map(|a| a.into_iter())
            .collect();
        fb.blit_rgba(0, 0, 8, 8, &patch).unwrap();
        fb.apply_surface_to_cache(1, 7, 0, 0, 8, 8).unwrap();
        assert!(fb.bitmap_cache.contains_key(&7));

        fb.resize_preserving_canvases(128, 128).unwrap();

        assert!(
            fb.bitmap_cache.contains_key(&7),
            "bitmap cache must survive a DisplayControl resize; the server does not resend SurfaceToCache"
        );
    }

    /// Shrink (a monitor removed) must keep the overlapping region's pixels.
    #[test]
    fn resize_shrink_preserves_overlapping_content() {
        let mut fb = Framebuffer::new(1003, 1004, 200, 80).unwrap();
        let green: Vec<u8> = std::iter::repeat([0u8, 255, 0, 0xff])
            .take(16)
            .flat_map(|a| a.into_iter())
            .collect();
        fb.blit_rgba(5, 5, 4, 4, &green).unwrap();

        fb.resize_preserving_canvases(100, 50).unwrap();

        let w = 100usize;
        let off = (5 * w + 5) * 4;
        let d = fb.image.data();
        assert_eq!(
            &d[off..off + 4],
            &[0, 255, 0, 0xff],
            "overlapping content must survive a shrink resize"
        );
    }

    /// The server pointer cache lives inside the fast-path `Processor`. A
    /// `CachedPointer` PDU replays a previously-sent cursor bitmap by cache
    /// slot — so a mid-session resize must NOT rebuild the processor, or the
    /// cursor turns into the wrong bitmap / vanishes right after adding a
    /// monitor (the `CachedPointer` misses the emptied cache). Drives real
    /// fast-path pointer PDUs through `apply_fast_path` across a resize.
    #[test]
    fn resize_preserves_pointer_cache() {
        use ironrdp_pdu::fast_path::{EncryptionFlags, Fragmentation};
        use ironrdp_pdu::pointer::{
            CachedPointerAttribute, ColorPointerAttribute, Point16, PointerAttribute,
        };

        // A validated 1bpp 7×7 cursor (masks lifted verbatim from ironrdp's own
        // pointer testsuite, so decode is known-good).
        const AND_MASK: &[u8] = &[
            0b0111_1110, 0, 0b1001_1110, 0, 0b1000_0110, 0, 0b1100_0010, 0, 0b1100_0110, 0,
            0b1110_1010, 0, 0b1111_1100, 0,
        ];
        const XOR_MASK: &[u8] = &[
            0, 0, 0, 0, 0b0010_0000, 0, 0b0001_0000, 0, 0, 0, 0, 0, 0, 0,
        ];
        const SLOT: u16 = 3;

        // Wrap update bytes in a single-fragment fast-path update PDU + header,
        // exactly as `Processor::process` expects to parse them.
        let build = |update_code: UpdateCode, data: &[u8]| -> Vec<u8> {
            let pdu = FastPathUpdatePdu {
                fragmentation: Fragmentation::Single,
                update_code,
                compression_flags: None,
                compression_type: None,
                data,
            };
            let pdu_bytes = ironrdp_core::encode_vec(&pdu).unwrap();
            let header = FastPathHeader::new(EncryptionFlags::empty(), pdu_bytes.len());
            let mut out = ironrdp_core::encode_vec(&header).unwrap();
            out.extend_from_slice(&pdu_bytes);
            out
        };

        let new_pointer = PointerAttribute {
            xor_bpp: 1,
            color_pointer: ColorPointerAttribute {
                cache_index: SLOT,
                hot_spot: Point16 { x: 0, y: 0 },
                width: 7,
                height: 7,
                xor_mask: XOR_MASK,
                and_mask: AND_MASK,
            },
        };
        let new_data = ironrdp_core::encode_vec(&new_pointer).unwrap();
        let cached_data =
            ironrdp_core::encode_vec(&CachedPointerAttribute { cache_index: SLOT }).unwrap();

        let mut fb = Framebuffer::new(1003, 1004, 100, 50).unwrap();

        // Drive the processor directly (the `apply_fast_path` wrapper calls
        // `perf::performance_now()`, which panics on the non-wasm test target);
        // the pointer cache lives in `self.fast_path`, so this exercises the
        // exact behavior under test. `fast_path` and `image` are disjoint
        // fields, so the simultaneous &mut borrows are allowed.
        let mut output = WriteBuf::new();
        let updates = fb
            .fast_path
            .process(&mut fb.image, &build(UpdateCode::NewPointer, &new_data), &mut output)
            .unwrap();
        assert!(
            updates
                .iter()
                .any(|u| matches!(u, UpdateKind::PointerBitmap(_))),
            "NewPointer should emit a cursor bitmap and populate the cache"
        );

        // Mid-session DisplayControl resize (e.g. a monitor added).
        fb.resize_preserving_canvases(200, 80).unwrap();

        // A CachedPointer referencing the pre-resize slot must still resolve —
        // i.e. the pointer cache survived the resize.
        let mut output = WriteBuf::new();
        let updates = fb
            .fast_path
            .process(&mut fb.image, &build(UpdateCode::CachedPointer, &cached_data), &mut output)
            .unwrap();
        assert!(
            updates
                .iter()
                .any(|u| matches!(u, UpdateKind::PointerBitmap(_))),
            "CachedPointer after resize must replay the cached bitmap (pointer cache must survive the resize)"
        );
    }
}
