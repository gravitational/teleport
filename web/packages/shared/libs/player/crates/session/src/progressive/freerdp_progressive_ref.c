/*
 * FreeRDP RFX Progressive reference decoder for the oracle differential.
 *
 * Compiled only under `--features freerdp-oracle` (see ../../build.rs); never
 * part of the wasm bundle. Exposes ONE simple entry point so the Rust harness
 * (progressive/mod.rs) doesn't have to FFI FreeRDP's full progressive/region
 * API (which varies across versions). The C compiler validates these calls
 * against the real installed FreeRDP3 headers.
 *
 * If your FreeRDP version names things differently, this single file is the
 * only place to adjust (e.g. PIXEL_FORMAT_RGBA32 vs PIXEL_FORMAT_ABGR32, or
 * progressive_context_new signature).
 */
#include <stdint.h>

#include <winpr/wtypes.h>
#include <freerdp/codec/color.h>
#include <freerdp/codec/region.h>
#include <freerdp/codec/progressive.h>

/*
 * Decode a sequence of WireToSurface2 progressive payloads, in order, into
 * `out_rgba` (width*height*4, RGBA byte order). State persists across the PDUs
 * within this call (one PROGRESSIVE_CONTEXT), matching the live stateful
 * decoder. Pass `n_pdus = k` to decode only the first k PDUs (the harness uses
 * this to localize the first frame where our decode diverges).
 *
 * Returns 0 on success; -1/-2 on context setup failure; -(100+i) if
 * progressive_decompress failed on PDU i.
 */
int freerdp_progressive_decode_sequence(const uint8_t* const* pdus, const uint32_t* sizes,
                                        uint32_t n_pdus, uint16_t surface_id, uint32_t width,
                                        uint32_t height, uint8_t* out_rgba)
{
	PROGRESSIVE_CONTEXT* ctx = progressive_context_new(FALSE /* decoder, not compressor */);
	if (!ctx)
		return -1;

	if (progressive_create_surface_context(ctx, (UINT16)surface_id, width, height) < 0)
	{
		progressive_context_free(ctx);
		return -2;
	}

	const UINT32 stride = width * 4u;
	int result = 0;
	for (uint32_t i = 0; i < n_pdus; i++)
	{
		REGION16 invalid;
		region16_init(&invalid);
		int rc = progressive_decompress(ctx, pdus[i], sizes[i], out_rgba, PIXEL_FORMAT_RGBA32,
		                                stride, 0 /* nXDst */, 0 /* nYDst */, &invalid,
		                                (UINT16)surface_id, i /* frameId */);
		region16_uninit(&invalid);
		if (rc < 0)
		{
			result = -100 - (int)i;
			break;
		}
	}

	progressive_context_free(ctx);
	return result;
}
