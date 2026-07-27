/**
 * GPU plumbing throughput microbench for the RFX-Progressive IDWT+color offload.
 *
 * The stage-split measurement already tells us IDWT+color = ~64% of per-tile CPU
 * decode, so moving them to the GPU *should* free the CPU pool. The one thing it
 * can't tell us — and the moonshot's biggest risk — is whether the GPU PLUMBING
 * (per-frame coefficient upload + the ping-pong IDWT passes + composite) can keep
 * up with the tile rate inside the present budget. This measures exactly that, on
 * the real path: signed-integer textures (the bit-exact format the IDWT needs —
 * R16I by default since the coefficients are natively i16, R32I selectable via
 * `bits` to A/B the upload cost), a lifting-shaped dummy pass, and the ACTUAL
 * color GLSL as the composite (so it also smoke-tests the color math on a real GPU).
 *
 * It runs in the worker (where the production GL lives) on its OWN OffscreenCanvas
 * so it never touches the live framebuffer. Triggered from the main console via
 * `await gpuBench()` (see DesktopSessionTestMulti).
 *
 * NOTE: a plumbing bench, not a correctness test — the dummy pass does realistic
 * lifting-shaped ALU/fetches but not the real IDWT; pixels are not validated here
 * (that's Phase A's diff-test). Numbers are an UPPER BOUND: it runs every pass
 * over the full 64×64 tile, whereas the real IDWT works on shrinking sub-regions.
 */

export interface GpuBenchResult {
  ok: boolean;
  error?: string;
  numTiles: number;
  atlas: string;
  /** Coefficient texture bit-depth (16 = R16I, native i16; 32 = R32I baseline). */
  bits: number;
  passesPerComponent: number;
  iters: number;
  totalMsPerFrame: number;
  uploadMsPerFrame: number;
  passesMsPerFrame: number;
  tilesPerSec: number;
  coeffMiBPerFrame: number;
  /** How many 1600-tile frames fit in a 22 ms (~45fps) present budget. */
  framesPer22ms: number;
}

const VERT = `#version 300 es
// Fullscreen triangle from gl_VertexID (no vertex buffer).
void main() {
  vec2 p = vec2(float((gl_VertexID << 1) & 2), float(gl_VertexID & 2));
  gl_Position = vec4(p * 2.0 - 1.0, 0.0, 1.0);
}`;

// Lifting-shaped integer pass: read self + two neighbors, combine with the
// predict/floor the real 5/3 inverse uses. Output to an R32I attachment.
const PASS_FRAG = `#version 300 es
precision highp int;
precision highp float;
uniform highp isampler2D u_src;
out highp int o;
void main() {
  ivec2 c = ivec2(gl_FragCoord.xy);
  int a = texelFetch(u_src, c, 0).r;
  int b = texelFetch(u_src, c + ivec2(1, 0), 0).r;
  int d = texelFetch(u_src, c + ivec2(0, 1), 0).r;
  o = a - ((b + d + 1) >> 1) + (a << 1);
}`;

// The REAL color pass (matches gpu_ref::color_pass / ironrdp ycbcr_to_rgb),
// composited to the RGBA8 canvas — so this bench also exercises the color GLSL.
const COMPOSITE_FRAG = `#version 300 es
precision highp int;
precision highp float;
uniform highp isampler2D u_y;
uniform highp isampler2D u_cb;
uniform highp isampler2D u_cr;
out vec4 o;
void main() {
  ivec2 c = ivec2(gl_FragCoord.xy);
  int y = texelFetch(u_y, c, 0).r & 0xffff;
  int cb = texelFetch(u_cb, c, 0).r & 0xffff;
  int cr = texelFetch(u_cr, c, 0).r & 0xffff;
  int yv = (y + 4096) << 16;
  int r = ((cr * 91916 + yv) >> 21) & 0xffff;
  int g = ((yv - cb * 22527 - cr * 46819) >> 21) & 0xffff;
  int b = ((cb * 115992 + yv) >> 21) & 0xffff;
  o = vec4(
    float(clamp(r, 0, 255)) / 255.0,
    float(clamp(g, 0, 255)) / 255.0,
    float(clamp(b, 0, 255)) / 255.0,
    1.0);
}`;

function compile(gl: WebGL2RenderingContext, type: number, src: string): WebGLShader {
  const s = gl.createShader(type)!;
  gl.shaderSource(s, src);
  gl.compileShader(s);
  if (!gl.getShaderParameter(s, gl.COMPILE_STATUS)) {
    throw new Error(`shader compile: ${gl.getShaderInfoLog(s)}`);
  }
  return s;
}

function link(gl: WebGL2RenderingContext, frag: string): WebGLProgram {
  const p = gl.createProgram()!;
  gl.attachShader(p, compile(gl, gl.VERTEX_SHADER, VERT));
  gl.attachShader(p, compile(gl, gl.FRAGMENT_SHADER, frag));
  gl.linkProgram(p);
  if (!gl.getProgramParameter(p, gl.LINK_STATUS)) {
    throw new Error(`program link: ${gl.getProgramInfoLog(p)}`);
  }
  return p;
}

function makeIntTex(
  gl: WebGL2RenderingContext,
  w: number,
  h: number,
  internalFormat: number,
  type: number,
  data: Int16Array | Int32Array | null
) {
  const tex = gl.createTexture()!;
  gl.bindTexture(gl.TEXTURE_2D, tex);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
  gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
  // Both R16I and R32I are color-renderable + texturable in core WebGL2.
  gl.texImage2D(gl.TEXTURE_2D, 0, internalFormat, w, h, 0, gl.RED_INTEGER, type, data);
  return tex;
}

export function runGpuPlumbingBench(
  numTiles = 1600,
  passesPerComponent = 12,
  iters = 20,
  bits: 16 | 32 = 16
): GpuBenchResult {
  const base: GpuBenchResult = {
    ok: false,
    numTiles,
    atlas: '',
    bits,
    passesPerComponent,
    iters,
    totalMsPerFrame: 0,
    uploadMsPerFrame: 0,
    passesMsPerFrame: 0,
    tilesPerSec: 0,
    coeffMiBPerFrame: 0,
    framesPer22ms: 0,
  };
  let ctxForCleanup: WebGL2RenderingContext | null = null;
  try {
    const side = Math.ceil(Math.sqrt(numTiles));
    const w = side * 64;
    const h = side * 64;
    base.atlas = `${w}x${h}`;
    const canvas = new OffscreenCanvas(w, h);
    const gl = canvas.getContext('webgl2', {
      antialias: false,
      alpha: false,
      preserveDrawingBuffer: false,
    });
    if (!gl) return { ...base, error: 'no webgl2 context' };
    ctxForCleanup = gl;

    // Coefficient texture format: R16I (native i16, half the upload) or R32I baseline.
    const internalFormat = bits === 16 ? gl.R16I : gl.R32I;
    const pixelType = bits === 16 ? gl.SHORT : gl.INT;

    const passProg = link(gl, PASS_FRAG);
    const compProg = link(gl, COMPOSITE_FRAG);
    const uSrc = gl.getUniformLocation(passProg, 'u_src');
    const uY = gl.getUniformLocation(compProg, 'u_y');
    const uCb = gl.getUniformLocation(compProg, 'u_cb');
    const uCr = gl.getUniformLocation(compProg, 'u_cr');

    // 3 input planes (Y/Cb/Cr) of coefficients, uploaded each frame. The pseudo-random
    // values are already in i16 range, so the 16- and 32-bit runs upload identical data
    // (only the wire width differs) — a clean A/B of upload cost.
    const planeData: Array<Int16Array | Int32Array> = [0, 1, 2].map(() => {
      const d = bits === 16 ? new Int16Array(w * h) : new Int32Array(w * h);
      for (let i = 0; i < d.length; i++) d[i] = ((i * 2654435761) | 0) >> 16; // cheap pseudo-random i16-ish
      return d;
    });
    const inputs = [0, 1, 2].map(() => makeIntTex(gl, w, h, internalFormat, pixelType, null));

    // Ping-pong pair for the IDWT passes.
    const ping = makeIntTex(gl, w, h, internalFormat, pixelType, null);
    const pong = makeIntTex(gl, w, h, internalFormat, pixelType, null);
    const fbo = gl.createFramebuffer()!;
    const vao = gl.createVertexArray()!;
    gl.bindVertexArray(vao);
    gl.disable(gl.DEPTH_TEST);
    gl.disable(gl.BLEND);
    const sync = new Uint8Array(4);

    // Fail loudly if the chosen integer format isn't renderable as a color attachment
    // (rather than silently producing bogus timings from no-op draws).
    gl.bindFramebuffer(gl.FRAMEBUFFER, fbo);
    gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, ping, 0);
    const fbStatus = gl.checkFramebufferStatus(gl.FRAMEBUFFER);
    gl.bindFramebuffer(gl.FRAMEBUFFER, null);
    if (fbStatus !== gl.FRAMEBUFFER_COMPLETE) {
      return {
        ...base,
        error: `R${bits}I not framebuffer-complete (status 0x${fbStatus.toString(16)})`,
      };
    }

    const uploadInputs = () => {
      for (let i = 0; i < 3; i++) {
        gl.bindTexture(gl.TEXTURE_2D, inputs[i]);
        gl.texImage2D(gl.TEXTURE_2D, 0, internalFormat, w, h, 0, gl.RED_INTEGER, pixelType, planeData[i]);
      }
    };

    const runPasses = () => {
      gl.viewport(0, 0, w, h);
      gl.useProgram(passProg);
      gl.uniform1i(uSrc, 0);
      gl.activeTexture(gl.TEXTURE0);
      // 3 components × N ping-pong passes (the full per-frame IDWT pass count).
      for (let comp = 0; comp < 3; comp++) {
        let src = inputs[comp];
        let dst = ping;
        let other = pong;
        for (let p = 0; p < passesPerComponent; p++) {
          gl.bindFramebuffer(gl.FRAMEBUFFER, fbo);
          gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, dst, 0);
          gl.bindTexture(gl.TEXTURE_2D, src);
          gl.drawArrays(gl.TRIANGLES, 0, 3);
          src = dst;
          const t = dst;
          dst = other;
          other = t;
        }
      }
    };

    const composite = () => {
      gl.bindFramebuffer(gl.FRAMEBUFFER, null);
      gl.viewport(0, 0, w, h);
      gl.useProgram(compProg);
      gl.uniform1i(uY, 0);
      gl.uniform1i(uCb, 1);
      gl.uniform1i(uCr, 2);
      for (let i = 0; i < 3; i++) {
        gl.activeTexture(gl.TEXTURE0 + i);
        gl.bindTexture(gl.TEXTURE_2D, inputs[i]);
      }
      gl.drawArrays(gl.TRIANGLES, 0, 3);
    };

    const forceSync = () => {
      // readPixels forces the pipeline to drain — a real (if coarse) per-frame sync.
      gl.bindFramebuffer(gl.FRAMEBUFFER, null);
      gl.readPixels(0, 0, 1, 1, gl.RGBA, gl.UNSIGNED_BYTE, sync);
    };

    // Warmup (shader/texture residency, JIT).
    for (let i = 0; i < 3; i++) {
      uploadInputs();
      runPasses();
      composite();
      forceSync();
    }

    // Full frame: upload + passes + composite.
    let t0 = performance.now();
    for (let i = 0; i < iters; i++) {
      uploadInputs();
      runPasses();
      composite();
      forceSync();
    }
    const totalMsPerFrame = (performance.now() - t0) / iters;

    // Upload-dominant: upload + a single trivial composite (no IDWT passes).
    t0 = performance.now();
    for (let i = 0; i < iters; i++) {
      uploadInputs();
      composite();
      forceSync();
    }
    const uploadMsPerFrame = (performance.now() - t0) / iters;

    const coeffMiBPerFrame = (numTiles * 64 * 64 * 3 * (bits / 8)) / (1024 * 1024);
    return {
      ...base,
      ok: true,
      totalMsPerFrame,
      uploadMsPerFrame,
      passesMsPerFrame: Math.max(0, totalMsPerFrame - uploadMsPerFrame),
      tilesPerSec: totalMsPerFrame > 0 ? (numTiles / totalMsPerFrame) * 1000 : 0,
      coeffMiBPerFrame,
      framesPer22ms: totalMsPerFrame > 0 ? 22 / totalMsPerFrame : 0,
    };
  } catch (e) {
    return { ...base, error: e instanceof Error ? e.message : String(e) };
  } finally {
    // Free the context so repeated calls (e.g. a numTiles sweep) don't exhaust the
    // browser's WebGL context pool ("Too many active WebGL contexts").
    ctxForCleanup?.getExtension('WEBGL_lose_context')?.loseContext();
  }
}

/** One-line summary for the console / logsink. */
export function formatGpuBench(r: GpuBenchResult): string {
  if (!r.ok) return `[gpubench] FAILED: ${r.error}`;
  return (
    `[gpubench] R${r.bits}I ${r.numTiles} tiles @ ${r.atlas}, ${r.passesPerComponent}×3 passes, ${r.iters} iters: ` +
    `total=${r.totalMsPerFrame.toFixed(2)}ms/frame ` +
    `(upload≈${r.uploadMsPerFrame.toFixed(2)} passes≈${r.passesMsPerFrame.toFixed(2)}) ` +
    `→ ${(r.tilesPerSec / 1000).toFixed(0)}k tiles/s, ${r.coeffMiBPerFrame.toFixed(1)}MiB/frame upload, ` +
    `${r.framesPer22ms.toFixed(1)} such frames per 22ms budget`
  );
}
