//! Native integer references for the Phase-A GPU IDWT + color offload.
//!
//! Each function mirrors, op-for-op, what the WebGL2 fragment shaders will
//! compute — written in the GLSL-`int` style (explicit wrapping i32 arithmetic,
//! explicit arithmetic shifts, explicit i16-wrap via `(v << 16) >> 16`) so the
//! shader is a 1:1 port of this Rust. They double as the bit-exactness oracle:
//! the tests assert `max|Δ| == 0` against ironrdp's CPU decode, so a shader that
//! matches this reference matches FreeRDP / mstsc. The difference/upgrade chain
//! stays CPU-side (coefficient domain), so this stage is stateless per-frame.
//!
//! Phase A is verification-only (no GPU yet); these run under `cargo test`.

/// `(INT16)` truncation = wrap to the low 16 bits, sign-extended — `as i16`
/// written the way GLSL (no 16-bit type) must express it.
#[inline]
fn wrap_i16(v: i32) -> i32 {
    (v << 16) >> 16
}

#[inline]
#[allow(clippy::cast_possible_truncation, clippy::cast_sign_loss)]
fn clamp_u8(v: i32) -> u8 {
    v.clamp(0, 255) as u8
}

/// YCbCr → RGBA color pass: the GLSL transliteration of ironrdp's
/// `ironrdp_graphics::progressive::ycbcr_to_rgb` (a FreeRDP-bit-exact port).
/// Inputs are post-IDWT spatial samples (i16 range); output is RGBA8 with opaque
/// alpha. All products wrap in i32 BY DESIGN (`cr * 91916` overflows i32); the
/// `(INT16)` cast is applied AFTER folding `>>16 >>5` into `>>21`.
#[allow(clippy::cast_possible_wrap, clippy::cast_sign_loss)]
pub(crate) fn color_pass(y: i32, cb: i32, cr: i32) -> [u8; 4] {
    // `yv` folds the +128 level shift through the later >>5 via +4096; the u32
    // cast makes the <<16 wrap explicit/defined, exactly as the oracle does.
    let yv = (((y + 4096) as u32) << 16) as i32;
    let r = wrap_i16((cr.wrapping_mul(91916).wrapping_add(yv)) >> 21);
    let g = wrap_i16(
        (yv.wrapping_sub(cb.wrapping_mul(22527))
            .wrapping_sub(cr.wrapping_mul(46819)))
            >> 21,
    );
    let b = wrap_i16((cb.wrapping_mul(115992).wrapping_add(yv)) >> 21);
    [clamp_u8(r), clamp_u8(g), clamp_u8(b), 0xFF]
}

#[cfg(test)]
mod tests {
    use super::*;

    /// The color pass must be byte-exact with ironrdp's `ycbcr_to_rgb` (the
    /// FreeRDP-bit-exact oracle) across the input domain — including negative
    /// coefficients, the overflow-by-design products, and the >>21 / i16-wrap
    /// boundaries. `max|Δ| == 0` is the Phase-A gate.
    #[test]
    #[allow(clippy::cast_possible_truncation)]
    fn color_pass_byte_exact_vs_ironrdp_oracle() {
        let mut vals: Vec<i32> = (-32768..=32767).step_by(1021).collect();
        // Dense edge/boundary coverage where wrap/shift bugs hide.
        vals.extend([
            -32768, -32767, -32766, -16385, -16384, -4097, -4096, -257, -256, -129, -128, -1, 0, 1,
            127, 128, 255, 256, 4095, 4096, 4097, 16383, 16384, 32766, 32767,
        ]);
        let mut max_delta = 0i32;
        for &y in &vals {
            for &cb in &vals {
                for &cr in &vals {
                    let got = color_pass(y, cb, cr);
                    let (r, g, b) =
                        ironrdp_graphics::progressive::ycbcr_to_rgb(y as i16, cb as i16, cr as i16);
                    assert_eq!(got[3], 0xFF, "alpha must be opaque");
                    max_delta = max_delta
                        .max((i32::from(got[0]) - i32::from(r)).abs())
                        .max((i32::from(got[1]) - i32::from(g)).abs())
                        .max((i32::from(got[2]) - i32::from(b)).abs());
                }
            }
        }
        assert_eq!(
            max_delta, 0,
            "GPU color pass must be byte-exact with the ironrdp CPU oracle"
        );
    }
}
