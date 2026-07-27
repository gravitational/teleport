/**
 * Copyright 2025 Gravitational, Inc.
 *
 * WebGL2 painter for a desktop-region framebuffer, used by pop-out display
 * windows. Mirrors the wasm-side `gl.rs` painter: the canvas content lives in an
 * `RGBA8` texture sized to the canvas; each region update is uploaded via
 * `texSubImage2D` (straight from the received pixel buffer, no `ImageData`
 * round-trip) and a single textured fullscreen quad is drawn. This replaces the
 * canvas2D `putImageData` path, matching the main display's pipeline.
 */

// Fullscreen quad from gl_VertexID (no vertex buffer). UV y is flipped so
// texture row 0 ends up at the canvas top — same convention as gl.rs.
const VERT_SRC = `#version 300 es
const vec2 QUAD_POS[6] = vec2[6](
  vec2(-1.0, -1.0), vec2( 1.0, -1.0), vec2( 1.0,  1.0),
  vec2(-1.0, -1.0), vec2( 1.0,  1.0), vec2(-1.0,  1.0)
);
const vec2 QUAD_UV[6] = vec2[6](
  vec2(0.0, 1.0), vec2(1.0, 1.0), vec2(1.0, 0.0),
  vec2(0.0, 1.0), vec2(1.0, 0.0), vec2(0.0, 0.0)
);
out vec2 v_uv;
void main() {
  v_uv = QUAD_UV[gl_VertexID];
  gl_Position = vec4(QUAD_POS[gl_VertexID], 0.0, 1.0);
}
`;

const FRAG_SRC = `#version 300 es
precision highp float;
uniform sampler2D u_tex;
in vec2 v_uv;
out vec4 fragColor;
void main() { fragColor = texture(u_tex, v_uv); }
`;

function compileShader(
  gl: WebGL2RenderingContext,
  kind: number,
  src: string
): WebGLShader {
  const shader = gl.createShader(kind);
  if (!shader) throw new Error('createShader failed');
  gl.shaderSource(shader, src);
  gl.compileShader(shader);
  if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
    const log = gl.getShaderInfoLog(shader);
    throw new Error(`shader compile failed: ${log}`);
  }
  return shader;
}

function buildProgram(gl: WebGL2RenderingContext): WebGLProgram {
  const vs = compileShader(gl, gl.VERTEX_SHADER, VERT_SRC);
  const fs = compileShader(gl, gl.FRAGMENT_SHADER, FRAG_SRC);
  const program = gl.createProgram();
  if (!program) throw new Error('createProgram failed');
  gl.attachShader(program, vs);
  gl.attachShader(program, fs);
  gl.linkProgram(program);
  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
    const log = gl.getProgramInfoLog(program);
    throw new Error(`program link failed: ${log}`);
  }
  gl.deleteShader(vs);
  gl.deleteShader(fs);
  return program;
}

export class GlFramebufferPainter {
  private gl: WebGL2RenderingContext;
  private program: WebGLProgram;
  private texture: WebGLTexture;
  private width = 0;
  private height = 0;

  constructor(canvas: HTMLCanvasElement) {
    // preserveDrawingBuffer: we only upload the changed region and rely on the
    // texture + drawn quad persisting across commits (partial-update semantics).
    // alpha/antialias off — 1:1 integer pixel content, no AA or alpha needed.
    const gl = canvas.getContext('webgl2', {
      preserveDrawingBuffer: true,
      alpha: false,
      antialias: false,
    });
    if (!gl) throw new Error("getContext('webgl2') returned null");
    this.gl = gl;
    this.program = buildProgram(gl);

    const texture = gl.createTexture();
    if (!texture) throw new Error('createTexture failed');
    this.texture = texture;

    // WebGL2 needs a bound VAO for drawArrays even with no attributes.
    const vao = gl.createVertexArray();
    gl.bindVertexArray(vao);

    gl.useProgram(this.program);
    gl.uniform1i(gl.getUniformLocation(this.program, 'u_tex'), 0);
    gl.activeTexture(gl.TEXTURE0);
    gl.bindTexture(gl.TEXTURE_2D, this.texture);
    gl.disable(gl.DEPTH_TEST);
    gl.disable(gl.BLEND);
    gl.pixelStorei(gl.UNPACK_PREMULTIPLY_ALPHA_WEBGL, 0);
    gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, 0);
    gl.pixelStorei(gl.UNPACK_COLORSPACE_CONVERSION_WEBGL, gl.NONE);

    this.resize(canvas.width, canvas.height);
  }

  /** Reallocate the texture + GL viewport. Cheap; called on size changes. */
  resize(width: number, height: number): void {
    if (width <= 0 || height <= 0) return;
    const gl = this.gl;
    gl.viewport(0, 0, width, height);
    // Immutable RGBA8 storage; once allocated we only texSubImage2D into it.
    gl.deleteTexture(this.texture);
    const tex = gl.createTexture();
    if (!tex) throw new Error('createTexture failed on resize');
    this.texture = tex;
    gl.bindTexture(gl.TEXTURE_2D, tex);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
    gl.texStorage2D(gl.TEXTURE_2D, 1, gl.RGBA8, width, height);
    this.width = width;
    this.height = height;
  }

  /**
   * Upload an `srcW x srcH` RGBA region (row-major, top-down) so its top-left
   * lands at canvas pixel (`dstX`, `dstY`), then redraw the quad. The region is
   * clipped to the canvas; the source sub-rect is selected with
   * UNPACK_ROW_LENGTH/SKIP so no intermediate copy is needed. `dstX`/`dstY` may
   * be negative (region starts off the left/top edge).
   */
  paint(rgba: Uint8Array, dstX: number, dstY: number, srcW: number, srcH: number): void {
    const gl = this.gl;
    if (this.width === 0 || this.height === 0) return;

    // Clip the destination rect to the texture, deriving the source sub-rect.
    const dx0 = Math.max(0, dstX);
    const dy0 = Math.max(0, dstY);
    const dx1 = Math.min(this.width, dstX + srcW);
    const dy1 = Math.min(this.height, dstY + srcH);
    const w = dx1 - dx0;
    const h = dy1 - dy0;
    if (w > 0 && h > 0) {
      const srcX = dx0 - dstX;
      const srcY = dy0 - dstY;
      // The texture is bound once (in the constructor / resize) and this
      // painter owns its context, so nothing unbinds it: texSubImage2D targets
      // the already-bound texture without a per-paint bindTexture.
      gl.pixelStorei(gl.UNPACK_ROW_LENGTH, srcW);
      gl.pixelStorei(gl.UNPACK_SKIP_PIXELS, srcX);
      gl.pixelStorei(gl.UNPACK_SKIP_ROWS, srcY);
      gl.texSubImage2D(
        gl.TEXTURE_2D,
        0,
        dx0,
        dy0,
        w,
        h,
        gl.RGBA,
        gl.UNSIGNED_BYTE,
        rgba
      );
      gl.pixelStorei(gl.UNPACK_ROW_LENGTH, 0);
      gl.pixelStorei(gl.UNPACK_SKIP_PIXELS, 0);
      gl.pixelStorei(gl.UNPACK_SKIP_ROWS, 0);
    }

    // Program already active from the constructor; nothing rebinds it.
    gl.drawArrays(gl.TRIANGLES, 0, 6);
  }

  dispose(): void {
    const gl = this.gl;
    gl.deleteTexture(this.texture);
    gl.deleteProgram(this.program);
  }
}
