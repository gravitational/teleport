//! Wire-order apply queue for offloaded EGFX paint ops.
//!
//! ## Why this exists
//! Offloading RFX-Progressive decode to the worker pool made its framebuffer
//! blit **asynchronous** — the decoded tile blob returns in a later task and is
//! composited then. The other EGFX paint ops (SurfaceToSurface, SurfaceToCache,
//! CacheToSurface, SolidFill, ClearCodec, Uncompressed, Planar, Bitmap) still
//! applied **synchronously** at PDU-parse time inside `feed_bytes`. That
//! reordered framebuffer mutations out of wire order: an inline
//! `SurfaceToSurface` (a window-move scroll/drag-copy) that the wire placed
//! *after* an RFX update ran *before* the RFX blob landed and read **stale**
//! pixels; `SurfaceToCache` then latched that staleness into the bitmap cache,
//! which a later `CacheToSurface` replayed — cumulative, non-self-healing
//! corruption while dragging a window.
//!
//! ## What it does
//! Restores strict wire order. In offload mode **every** paint op is assigned a
//! monotonic `seq` and enqueued in wire order:
//! - inline ops are *captured* (their params copied out of the PDU) and are
//!   immediately ready;
//! - RFX ops are *pending* until the decode worker(s) return their tile
//!   blob(s);
//! - the EGFX EndFrame boundary enqueues a [`PendingPaint::Present`] marker.
//!
//! [`PaintQueue::drain_ready`] yields the contiguous prefix of ready ops in
//! `seq` order and **stops at the first not-ready (pending RFX) entry** — so an
//! inline op never applies until all lower-`seq` RFX blits have landed, and no
//! higher-`seq` RFX blit applies before it. The caller applies the returned ops
//! to the framebuffer.
//!
//! The queue itself is pure data (no framebuffer / GL / wasm-bindgen deps) so
//! the ordering invariant is unit-testable natively.

use std::collections::VecDeque;

/// One captured EGFX paint operation, fully owned (no borrow of the decoded
/// PDU) so it can outlive `feed_bytes` and apply later, in order.
pub(crate) enum PendingPaint {
    /// RFX-Progressive. The worker pool returns one self-describing tile blob
    /// per position partition; `blobs` accumulates them (via
    /// [`PaintQueue::stage_rfx_blob`]) until the host signals the `seq` complete
    /// ([`PaintQueue::mark_rfx_ready`]). `origin_x/y` is the surface origin in
    /// desktop coords, applied per-tile at blit time.
    Rfx {
        origin_x: u32,
        origin_y: u32,
        blobs: Vec<Vec<u8>>,
    },
    /// EGFX SolidFill. `rects` are `(left, top, right, bottom)`.
    SolidFill {
        surface_id: u32,
        r: u8,
        g: u8,
        b: u8,
        rects: Vec<(u32, u32, u32, u32)>,
    },
    /// EGFX SurfaceToCache — snapshots `src` `(left, top, right, bottom)` of the
    /// surface into the bitmap cache. **Reads** the framebuffer image, so it
    /// must be ordered after prior RFX (this is the op that latched staleness).
    SurfaceToCache {
        surface_id: u32,
        cache_slot: u32,
        src: (u32, u32, u32, u32),
    },
    /// EGFX CacheToSurface — blits a cached region to each dest point.
    CacheToSurface {
        surface_id: u32,
        cache_slot: u32,
        points: Vec<(u32, u32)>,
    },
    /// EGFX SurfaceToSurface — copies `src` `(left, top, right, bottom)` to each
    /// dest point. **Reads** the framebuffer image (the drag/scroll copy).
    SurfaceToSurface {
        src_surface_id: u32,
        dst_surface_id: u32,
        src: (u32, u32, u32, u32),
        points: Vec<(u32, u32)>,
    },
    /// EGFX ClearCodec (raw `RFX_CLEAR_BITMAP_STREAM`).
    ClearCodec {
        surface_id: u32,
        dest_x: i32,
        dest_y: i32,
        width: u32,
        height: u32,
        pdu: Vec<u8>,
    },
    /// EGFX Uncompressed WireToSurface1 blit.
    Uncompressed {
        surface_id: u32,
        dest_x: i32,
        dest_y: i32,
        width: u32,
        height: u32,
        pixel_format: u32,
        bitmap: Vec<u8>,
    },
    /// EGFX Planar (RDP 6.0 bitmap stream).
    Planar {
        surface_id: u32,
        dest_x: i32,
        dest_y: i32,
        width: u32,
        height: u32,
        pdu: Vec<u8>,
    },
    /// Pre-decoded EGFX bitmap (RGBA already in desktop coords).
    Bitmap {
        desktop_x: u32,
        desktop_y: u32,
        width: u32,
        height: u32,
        rgba: Vec<u8>,
    },
    /// EGFX EndFrame boundary — flush/present everything applied so far.
    Present,
}

struct Entry {
    seq: u32,
    op: PendingPaint,
    /// Inline ops + `Present` are ready at enqueue; RFX becomes ready once the
    /// host has staged all its partition blobs.
    ready: bool,
}

/// FIFO, seq-ordered queue of pending EGFX paint ops. See module docs.
pub(crate) struct PaintQueue {
    next_seq: u32,
    entries: VecDeque<Entry>,
}

impl PaintQueue {
    pub(crate) fn new() -> Self {
        Self {
            next_seq: 0,
            entries: VecDeque::new(),
        }
    }

    /// Enqueue an RFX op, **pending** until [`Self::mark_rfx_ready`]. Returns its
    /// `seq` so the host can tag the dispatched chunk and its returning blobs.
    pub(crate) fn enqueue_rfx(&mut self, origin_x: u32, origin_y: u32) -> u32 {
        self.push(
            PendingPaint::Rfx {
                origin_x,
                origin_y,
                blobs: Vec::new(),
            },
            false,
        )
    }

    /// Enqueue an already-decoded inline op (or the [`PendingPaint::Present`]
    /// marker), **ready** immediately. Returns its `seq`.
    pub(crate) fn enqueue_ready(&mut self, op: PendingPaint) -> u32 {
        self.push(op, true)
    }

    fn push(&mut self, op: PendingPaint, ready: bool) -> u32 {
        let seq = self.next_seq;
        self.next_seq = self.next_seq.wrapping_add(1);
        self.entries.push_back(Entry { seq, op, ready });
        seq
    }

    /// Stash a returned RFX tile blob for `seq` (one per worker partition). A
    /// blob for an unknown/already-drained `seq` is dropped (benign — the host
    /// only stages before [`Self::mark_rfx_ready`], which happens before drain).
    pub(crate) fn stage_rfx_blob(&mut self, seq: u32, blob: Vec<u8>) {
        if let Some(e) = self.entries.iter_mut().find(|e| e.seq == seq) {
            if let PendingPaint::Rfx { blobs, .. } = &mut e.op {
                blobs.push(blob);
            }
        }
    }

    /// Mark an RFX `seq`'s blobs all-returned (the host counted every expected
    /// worker reply). A no-op for an unknown `seq` (already drained / cleared).
    pub(crate) fn mark_rfx_ready(&mut self, seq: u32) {
        if let Some(e) = self.entries.iter_mut().find(|e| e.seq == seq) {
            e.ready = true;
        }
    }

    /// Pop and return, in `seq` order, the contiguous prefix of ready ops.
    /// Stops at the first not-ready entry (a still-pending RFX): that head
    /// blocks every later op — the wire-order guarantee.
    pub(crate) fn drain_ready(&mut self) -> Vec<PendingPaint> {
        let mut out = Vec::new();
        while self.entries.front().is_some_and(|e| e.ready) {
            out.push(self.entries.pop_front().unwrap().op);
        }
        out
    }

    /// Drop every queued entry (e.g. on resize / offload-off / teardown). The
    /// caller should [`Self::drain_ready`] + apply first if ready ops must not
    /// be lost; the remaining pending RFX can never complete after a reset.
    pub(crate) fn clear(&mut self) {
        self.entries.clear();
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Compact tag of each drained op, for order assertions.
    fn kinds(ops: &[PendingPaint]) -> Vec<&'static str> {
        ops.iter()
            .map(|o| match o {
                PendingPaint::Rfx { .. } => "rfx",
                PendingPaint::SolidFill { .. } => "solid",
                PendingPaint::SurfaceToCache { .. } => "s2c",
                PendingPaint::CacheToSurface { .. } => "c2s",
                PendingPaint::SurfaceToSurface { .. } => "s2s",
                PendingPaint::ClearCodec { .. } => "clear",
                PendingPaint::Uncompressed { .. } => "uncompressed",
                PendingPaint::Planar { .. } => "planar",
                PendingPaint::Bitmap { .. } => "bitmap",
                PendingPaint::Present => "present",
            })
            .collect()
    }

    fn s2s() -> PendingPaint {
        PendingPaint::SurfaceToSurface {
            src_surface_id: 1,
            dst_surface_id: 1,
            src: (0, 0, 64, 64),
            points: vec![(64, 0)],
        }
    }

    /// THE BUG, in queue terms: wire order is [RFX paints region R] then
    /// [SurfaceToSurface that reads R]. The S2S must NOT apply until the RFX
    /// blob has landed — otherwise it copies stale pixels.
    #[test]
    fn inline_op_waits_for_prior_pending_rfx() {
        let mut q = PaintQueue::new();
        let rfx = q.enqueue_rfx(0, 0);
        q.enqueue_ready(s2s());

        // RFX has not returned: nothing drains — S2S is blocked behind it.
        assert!(
            q.drain_ready().is_empty(),
            "SurfaceToSurface must not apply before the prior in-flight RFX"
        );

        // RFX blob returns and is marked complete:
        q.stage_rfx_blob(rfx, vec![0u8; 4]);
        q.mark_rfx_ready(rfx);
        assert_eq!(
            kinds(&q.drain_ready()),
            ["rfx", "s2s"],
            "apply in wire order once ready: RFX then SurfaceToSurface"
        );
        assert!(q.is_empty_for_test());
    }

    /// A frame with no RFX (a pure inline drag frame) drains immediately —
    /// nothing pending blocks the head.
    #[test]
    fn inline_only_frame_drains_immediately() {
        let mut q = PaintQueue::new();
        q.enqueue_ready(PendingPaint::SolidFill {
            surface_id: 1,
            r: 1,
            g: 2,
            b: 3,
            rects: vec![(0, 0, 10, 10)],
        });
        q.enqueue_ready(s2s());
        q.enqueue_ready(PendingPaint::Present);
        assert_eq!(kinds(&q.drain_ready()), ["solid", "s2s", "present"]);
    }

    /// Head-of-line: a *later* RFX that becomes ready first must still wait for
    /// the earlier-seq RFX (and so must the Present after them).
    #[test]
    fn later_ready_rfx_waits_for_earlier_head() {
        let mut q = PaintQueue::new();
        let r0 = q.enqueue_rfx(0, 0);
        let r1 = q.enqueue_rfx(0, 0);
        q.enqueue_ready(PendingPaint::Present);

        q.mark_rfx_ready(r1); // the later one finishes decoding first
        assert!(
            q.drain_ready().is_empty(),
            "later RFX must not jump ahead of the still-pending earlier RFX"
        );

        q.mark_rfx_ready(r0);
        assert_eq!(kinds(&q.drain_ready()), ["rfx", "rfx", "present"]);
    }

    /// All N partition blobs for one seq are preserved and carried together.
    #[test]
    fn multiple_partition_blobs_accumulate() {
        let mut q = PaintQueue::new();
        let r = q.enqueue_rfx(10, 20);
        q.stage_rfx_blob(r, vec![1u8; 4]);
        q.stage_rfx_blob(r, vec![2u8; 4]);
        q.stage_rfx_blob(r, vec![3u8; 4]);
        q.mark_rfx_ready(r);

        let drained = q.drain_ready();
        assert_eq!(drained.len(), 1);
        match &drained[0] {
            PendingPaint::Rfx {
                origin_x,
                origin_y,
                blobs,
            } => {
                assert_eq!((*origin_x, *origin_y), (10, 20));
                assert_eq!(blobs.len(), 3, "every partition blob is preserved");
            }
            _ => panic!("expected Rfx"),
        }
    }

    /// Partial progress: the ready prefix drains, the rest resumes when its head
    /// becomes ready. Models pipelined frames (frame N+1 chunks already arriving).
    #[test]
    fn partial_drain_then_resume_in_order() {
        let mut q = PaintQueue::new();
        let r0 = q.enqueue_rfx(0, 0);
        let r1 = q.enqueue_rfx(0, 0);
        q.enqueue_ready(PendingPaint::Present);

        q.mark_rfx_ready(r0);
        assert_eq!(kinds(&q.drain_ready()), ["rfx"], "only the ready head drains");

        q.mark_rfx_ready(r1);
        assert_eq!(kinds(&q.drain_ready()), ["rfx", "present"]);
        assert!(q.is_empty_for_test());
    }

    /// A reset (resize / offload-off / teardown) drops pending entries so a
    /// never-completing RFX can't wedge the queue forever.
    #[test]
    fn clear_drops_pending() {
        let mut q = PaintQueue::new();
        q.enqueue_rfx(0, 0);
        q.enqueue_ready(s2s());
        q.clear();
        assert!(q.drain_ready().is_empty());
        assert!(q.is_empty_for_test());
        // Seqs keep advancing monotonically after a clear.
        assert_eq!(q.enqueue_rfx(0, 0), 2);
    }

    impl PaintQueue {
        fn is_empty_for_test(&self) -> bool {
            self.entries.is_empty()
        }
    }
}
