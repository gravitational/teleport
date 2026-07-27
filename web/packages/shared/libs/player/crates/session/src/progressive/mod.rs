//! RFX Progressive FreeRDP oracle (test-only; `--features freerdp-oracle`).
//!
//! Replays captured WireToSurface2 fixtures — produced by the always-on
//! server-side `progressive_fixture_capture.go`, default
//! `/tmp/teleport-rfx-fixtures` (override with `TELEPORT_RFX_FIXTURES`) —
//! through BOTH our [`ProgressiveDecoder`] and FreeRDP's `progressive_decompress`
//! (via the C wrapper in `freerdp_progressive_ref.c`), and reports the first
//! frame where the two diverge at a target pixel. Ground-truth check for the
//! "RFX progressive decodes early passes to black" bug.
//!
//! Run (host, FreeRDP installed; macOS: `brew install freerdp`):
//!   cargo test -p session --features freerdp-oracle rfx_progressive -- --nocapture

use std::collections::HashMap;
use std::fs;
use std::path::{Path, PathBuf};

use ironrdp_graphics::progressive::{ProgressiveDecoder, RegionRect};

// Geometry of the capture (3200x660 @ 200% in the repro). Adjust if you
// captured a different resolution — it drives the tile grid + FreeRDP surface.
const SURFACE_W: u16 = 3200;
const SURFACE_H: u16 = 660;

// Pixel we observed flickering black (framebuffer coords) -> tile (25, 5).
const TARGET_X: usize = 1600;
const TARGET_Y: usize = 330;

// Cap the O(n^2) per-prefix FreeRDP re-decode. The black appears in the first
// handful of passes; keep the repro short and this is plenty.
const MAX_CHECK: usize = 150;

extern "C" {
    fn freerdp_progressive_decode_sequence(
        pdus: *const *const u8,
        sizes: *const u32,
        n_pdus: u32,
        surface_id: u16,
        width: u32,
        height: u32,
        out_rgba: *mut u8,
    ) -> i32;
}

struct Fixture {
    surface_id: u32,
    ctx_id: u32,
    payload: Vec<u8>,
}

fn fixtures_dir() -> PathBuf {
    std::env::var_os("TELEPORT_RFX_FIXTURES")
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from("/tmp/teleport-rfx-fixtures"))
}

fn load_fixtures(dir: &Path) -> Vec<Fixture> {
    let manifest = fs::read_to_string(dir.join("manifest.tsv")).unwrap_or_else(|e| {
        panic!(
            "read {}/manifest.tsv: {e} (copy the fixtures out of the container first)",
            dir.display()
        )
    });
    let mut out = Vec::new();
    for line in manifest.lines() {
        // seq ts_ns surface codec_id ctx pixel_format origin_x origin_y byte_count filename
        let c: Vec<&str> = line.split('\t').collect();
        if c.len() < 10 {
            continue;
        }
        let surface_id: u32 = match c[2].parse() {
            Ok(v) => v,
            Err(_) => continue, // header row / malformed
        };
        let ctx_id: u32 = c[4].parse().unwrap_or(0);
        let payload =
            fs::read(dir.join(c[9])).unwrap_or_else(|e| panic!("read fixture {}: {e}", c[9]));
        out.push(Fixture { surface_id, ctx_id, payload });
    }
    out
}

/// Composite a 64x64 tile onto the surface, clipped to the region's update
/// rectangles — matching FreeRDP `update_tiles` (each tile is copied only where
/// it intersects the region rects; elsewhere the prior frame is kept).
fn blit_tile(surf: &mut [u8], w: usize, h: usize, tx: usize, ty: usize, tile: &[u8], clip: &[RegionRect]) {
    let (x0, y0) = (tx * 64, ty * 64);
    for r in clip {
        let rx0 = usize::from(r.x).max(x0);
        let ry0 = usize::from(r.y).max(y0);
        let rx1 = (usize::from(r.x) + usize::from(r.width)).min(x0 + 64).min(w);
        let ry1 = (usize::from(r.y) + usize::from(r.height)).min(y0 + 64).min(h);
        for y in ry0..ry1 {
            for x in rx0..rx1 {
                let s = ((y - y0) * 64 + (x - x0)) * 4;
                let d = (y * w + x) * 4;
                surf[d..d + 4].copy_from_slice(&tile[s..s + 4]);
            }
        }
    }
}

#[test]
fn rfx_progressive_matches_freerdp() {
    let dir = fixtures_dir();
    let all = load_fixtures(&dir);
    assert!(!all.is_empty(), "no fixtures in {}", dir.display());

    // Single-surface repro: drive off the first surface_id and filter to it so
    // FreeRDP's single surface context matches.
    let surface_id = all[0].surface_id;
    let fixtures: Vec<&Fixture> = all.iter().filter(|f| f.surface_id == surface_id).collect();
    let n = fixtures.len().min(MAX_CHECK);
    eprintln!(
        "[oracle] {} PDUs (checking first {n}), surface={surface_id}, target=({TARGET_X},{TARGET_Y}) tile=({},{})",
        fixtures.len(),
        TARGET_X / 64,
        TARGET_Y / 64
    );

    let (w, h) = (SURFACE_W as usize, SURFACE_H as usize);
    let tidx = (TARGET_Y * w + TARGET_X) * 4;

    // Our decoder is stateful and persists across PDUs (one per surface).
    let mut decoders: HashMap<u32, ProgressiveDecoder> = HashMap::new();
    let mut ours = vec![0u8; w * h * 4];
    let mut first_divergence: Option<usize> = None;

    for k in 0..n {
        let f = fixtures[k];

        // ours: incremental decode of PDU k, blit returned tiles into surface.
        let dec = decoders.entry(f.surface_id).or_insert_with(ProgressiveDecoder::new);
        match dec.decode_bitmap(f.ctx_id, SURFACE_W, SURFACE_H, &f.payload) {
            Ok(tiles) => {
                for t in &tiles {
                    blit_tile(&mut ours, w, h, t.x_idx as usize, t.y_idx as usize, &t.pixels, &t.clip_rects);
                }
            }
            Err(e) => panic!("our decode failed at PDU {k}: {e:?}"),
        }
        let our_px = [ours[tidx], ours[tidx + 1], ours[tidx + 2], ours[tidx + 3]];

        // FreeRDP: decode the first (k+1) PDUs fresh (C wrapper makes a new
        // context each call) and read the same pixel.
        let ptrs: Vec<*const u8> = fixtures[..=k].iter().map(|x| x.payload.as_ptr()).collect();
        let sizes: Vec<u32> = fixtures[..=k].iter().map(|x| x.payload.len() as u32).collect();
        let mut theirs = vec![0u8; w * h * 4];
        let rc = unsafe {
            freerdp_progressive_decode_sequence(
                ptrs.as_ptr(),
                sizes.as_ptr(),
                ptrs.len() as u32,
                surface_id as u16,
                SURFACE_W as u32,
                SURFACE_H as u32,
                theirs.as_mut_ptr(),
            )
        };
        assert!(rc == 0, "freerdp decode failed at PDU {k}: rc={rc}");
        let their_px = [theirs[tidx], theirs[tidx + 1], theirs[tidx + 2], theirs[tidx + 3]];

        if our_px[..3] != their_px[..3] {
            eprintln!("[oracle] PDU {k}: ours={our_px:?} freerdp={their_px:?}  <-- DIVERGE");
            first_divergence.get_or_insert(k);
        } else {
            eprintln!("[oracle] PDU {k}: match {our_px:?}");
        }
    }

    if let Some(k) = first_divergence {
        panic!(
            "RFX progressive diverges from FreeRDP at ({TARGET_X},{TARGET_Y}) starting PDU {k} \
             (see [oracle] lines). ours=(0,0,0) + freerdp non-black => our first-pass/DC decode is wrong."
        );
    }
    eprintln!("[oracle] no divergence at target across {n} PDUs — try a different TARGET tile");
}

/// Full-surface differential oracle: compares EVERY pixel of the whole surface
/// (all tiles) against FreeRDP at every PDU. Reports per-PDU mismatch counts and
/// the bounding box of differences (region-clipped compositing tends to cluster
/// them at tile edges where FreeRDP keeps the prior frame outside the update
/// region while we blit the whole tile).
#[test]
fn rfx_progressive_full_screen_matches_freerdp() {
    let dir = fixtures_dir();
    let all = load_fixtures(&dir);
    assert!(!all.is_empty(), "no fixtures in {}", dir.display());
    let surface_id = all[0].surface_id;
    let fixtures: Vec<&Fixture> = all.iter().filter(|f| f.surface_id == surface_id).collect();
    let n = fixtures.len().min(MAX_CHECK);
    let (w, h) = (SURFACE_W as usize, SURFACE_H as usize);
    eprintln!("[full] {n} PDUs, surface {w}x{h} ({} px/frame)", w * h);

    let mut decoders: HashMap<u32, ProgressiveDecoder> = HashMap::new();
    let mut ours = vec![0u8; w * h * 4];
    let mut total_mismatch_pdus = 0usize;
    let mut worst = (0usize, 0usize); // (pdu, mismatches)

    for k in 0..n {
        let f = fixtures[k];
        let dec = decoders.entry(f.surface_id).or_insert_with(ProgressiveDecoder::new);
        match dec.decode_bitmap(f.ctx_id, SURFACE_W, SURFACE_H, &f.payload) {
            Ok(tiles) => {
                for t in &tiles {
                    blit_tile(&mut ours, w, h, t.x_idx as usize, t.y_idx as usize, &t.pixels, &t.clip_rects);
                }
            }
            Err(e) => panic!("our decode failed at PDU {k}: {e:?}"),
        }

        let ptrs: Vec<*const u8> = fixtures[..=k].iter().map(|x| x.payload.as_ptr()).collect();
        let sizes: Vec<u32> = fixtures[..=k].iter().map(|x| x.payload.len() as u32).collect();
        let mut theirs = vec![0u8; w * h * 4];
        let rc = unsafe {
            freerdp_progressive_decode_sequence(
                ptrs.as_ptr(), sizes.as_ptr(), ptrs.len() as u32,
                surface_id as u16, SURFACE_W as u32, SURFACE_H as u32, theirs.as_mut_ptr(),
            )
        };
        assert!(rc == 0, "freerdp decode failed at PDU {k}: rc={rc}");

        let mut mismatches = 0usize;
        let mut max_d = 0i32;
        let (mut x0, mut y0, mut x1, mut y1) = (usize::MAX, usize::MAX, 0usize, 0usize);
        for y in 0..h {
            for x in 0..w {
                let i = (y * w + x) * 4;
                if ours[i..i + 3] != theirs[i..i + 3] {
                    mismatches += 1;
                    let d = (0..3).map(|c| (i32::from(ours[i + c]) - i32::from(theirs[i + c])).abs()).max().unwrap();
                    max_d = max_d.max(d);
                    x0 = x0.min(x); y0 = y0.min(y); x1 = x1.max(x); y1 = y1.max(y);
                }
            }
        }
        if mismatches == 0 {
            eprintln!("[full] PDU {k}: whole surface matches");
        } else {
            total_mismatch_pdus += 1;
            if mismatches > worst.1 { worst = (k, mismatches); }
            eprintln!(
                "[full] PDU {k}: {mismatches} px differ, max|Δ|={max_d}, bbox=({x0},{y0})..({x1},{y1}) [tiles ({}..{},{}..{})]",
                x0 / 64, x1 / 64, y0 / 64, y1 / 64
            );
        }
    }

    assert_eq!(
        total_mismatch_pdus, 0,
        "{total_mismatch_pdus}/{n} PDUs diverged from FreeRDP (worst PDU {} = {} px). \
         The whole surface must be pixel-identical to FreeRDP (decode + region-clipped compositing).",
        worst.0, worst.1
    );
    eprintln!("[full] whole surface matches FreeRDP across all {n} PDUs (every pixel)");
}
