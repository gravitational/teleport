//! WebGL2 painter for the decoded RDP framebuffer.
//!
//! Replaces the canvas2D `putImageData` path. The framebuffer image
//! lives in an `RGBA8` texture sized to the desktop resolution. On each
//! render we upload the dirty rows via `texSubImage2D` (a single bulk
//! copy from wasm linear memory into a GPU texture) and draw a single
//! textured fullscreen quad to the default framebuffer (the canvas
//! drawing buffer). The quad geometry comes from `gl_VertexID` so we
//! don't need any vertex buffer.
//!
//! Compared to canvas2D's pipeline:
//! - Removes the wasm→`Uint8ClampedArray`→`ImageData`→`putImageData`
//!   chain. Pixels go straight from wasm memory to the GPU.
//! - Eliminates the cached `ImageData`/`Uint8ClampedArray` pair we used
//!   to dodge per-frame JS allocations.
//! - Composite happens on the GPU, so paint cost stays sub-ms even when
//!   the dirty rect spans the whole screen.
//!
//! # Canvas targets
//!
//! The painter binds a WebGL2 context to either a main-thread
//! [`HtmlCanvasElement`] (the live-session case — the `<canvas>` lives in
//! the DOM) or an [`OffscreenCanvas`] (the worker / multi-monitor case,
//! where the canvas is transferred off the main thread). Both are handled
//! uniformly via [`PainterCanvas`]; nothing else in the painter is
//! monitor-aware. Multi-monitor is a layer *above* this: each monitor gets
//! its own painter and calls [`GlPainter::upload_rect`] with a viewport
//! offset. Single-monitor callers just pass `src == dst == 0`.

use std::cell::Cell;

use anyhow::{anyhow, Result};
use wasm_bindgen::JsCast;
use web_sys::{
    HtmlCanvasElement, OffscreenCanvas, WebGl2RenderingContext as Gl, WebGlProgram, WebGlShader,
    WebGlTexture, WebGlVertexArrayObject,
};

const VERT_SRC: &str = r#"#version 300 es
// Drives a single fullscreen quad from gl_VertexID — no vertex buffer.
// UV's y is flipped so texture row 0 ends up at canvas top.
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
"#;

const FRAG_SRC: &str = r#"#version 300 es
precision highp float;
uniform sampler2D u_tex;
in vec2 v_uv;
out vec4 fragColor;
void main() { fragColor = texture(u_tex, v_uv); }
"#;

/// A WebGL2-capable canvas: either a main-thread DOM canvas or an
/// `OffscreenCanvas` (worker / multi-monitor). Abstracts the two so the
/// painter body is target-agnostic.
enum PainterCanvas {
    Html(HtmlCanvasElement),
    // Worker / multi-monitor target (a transferred `OffscreenCanvas`). The
    // live single-canvas FastPath path only uses `Html`; this variant is the
    // optional off-main-thread surface, kept so a future multi-monitor layer
    // can drive one painter per transferred canvas without touching this file.
    #[allow(dead_code)]
    Offscreen(OffscreenCanvas),
}

impl PainterCanvas {
    fn get_context_webgl2(&self, attrs: &js_sys::Object) -> Result<Gl> {
        let ctx = match self {
            PainterCanvas::Html(c) => c.get_context_with_context_options("webgl2", attrs),
            PainterCanvas::Offscreen(c) => c.get_context_with_context_options("webgl2", attrs),
        }
        .map_err(|e| anyhow!("getContext('webgl2') failed: {e:?}"))?
        .ok_or_else(|| anyhow!("getContext('webgl2') returned null"))?;
        ctx.dyn_into::<Gl>()
            .map_err(|_| anyhow!("getContext returned non-WebGL2 object"))
    }

    fn width(&self) -> u32 {
        match self {
            PainterCanvas::Html(c) => c.width(),
            PainterCanvas::Offscreen(c) => c.width(),
        }
    }

    fn set_width(&self, w: u32) {
        match self {
            PainterCanvas::Html(c) => c.set_width(w),
            PainterCanvas::Offscreen(c) => c.set_width(w),
        }
    }

    fn height(&self) -> u32 {
        match self {
            PainterCanvas::Html(c) => c.height(),
            PainterCanvas::Offscreen(c) => c.height(),
        }
    }

    fn set_height(&self, h: u32) {
        match self {
            PainterCanvas::Html(c) => c.set_height(h),
            PainterCanvas::Offscreen(c) => c.set_height(h),
        }
    }
}

pub struct GlPainter {
    gl: Gl,
    // The canvas this painter owns. Kept so `resize` can adjust the drawing
    // buffer without a fragile `gl.canvas()` downcast (which type is it?).
    canvas: PainterCanvas,
    // Bound once in `new` and never re-bound (this painter owns the context).
    // Kept only to hold the JS handle alive for the painter's lifetime.
    _program: WebGlProgram,
    texture: WebGlTexture,
    _vao: WebGlVertexArrayObject,
    width: u32,
    height: u32,
    // Whether the quad has been drawn at least once since the last (re)size.
    // The first present always draws so the canvas paints even if its first
    // dirty rect missed this viewport; after that we only draw on upload.
    presented: Cell<bool>,
}

impl GlPainter {
    /// Attach a painter to a main-thread DOM `<canvas>` (the live-session
    /// case). The canvas must not already have a 2D context — a canvas can
    /// only ever hold one context type.
    pub fn new(canvas: HtmlCanvasElement, width: u32, height: u32) -> Result<Self> {
        Self::from_target(PainterCanvas::Html(canvas), width, height)
    }

    /// Attach a painter to an `OffscreenCanvas` (the worker / multi-monitor
    /// case, where the canvas is transferred off the main thread). Optional
    /// surface — not used by the live single-canvas FastPath path yet.
    #[allow(dead_code)]
    pub fn new_offscreen(canvas: OffscreenCanvas, width: u32, height: u32) -> Result<Self> {
        Self::from_target(PainterCanvas::Offscreen(canvas), width, height)
    }

    fn from_target(canvas: PainterCanvas, width: u32, height: u32) -> Result<Self> {
        // preserveDrawingBuffer=true is required for partial-update
        // semantics: we only upload the rows that changed and rely on
        // the texture (and the drawn quad) keeping their content across
        // commits. With the default (false), the drawing buffer is
        // erased after every implicit commit — any tick that didn't
        // call drawArrays would flash the canvas black, and PDUs
        // without a region update (frame markers, cursor-only) would
        // produce visible artifacts on drag.
        let attrs = js_sys::Object::new();
        let _ = js_sys::Reflect::set(
            &attrs,
            &"preserveDrawingBuffer".into(),
            &wasm_bindgen::JsValue::TRUE,
        );
        // alpha:false is a small win — the drawing buffer doesn't need
        // an alpha channel for RDP screen content, and disabling it
        // skips the per-commit alpha-premultiply pass some browsers do.
        let _ = js_sys::Reflect::set(&attrs, &"alpha".into(), &wasm_bindgen::JsValue::FALSE);
        // antialias:false — we render a 1:1 pixel texture, no MSAA
        // needed (and AA on integer pixel data is just blur).
        let _ = js_sys::Reflect::set(&attrs, &"antialias".into(), &wasm_bindgen::JsValue::FALSE);
        let gl = canvas.get_context_webgl2(&attrs)?;

        let program = build_program(&gl)?;
        let texture = gl
            .create_texture()
            .ok_or_else(|| anyhow!("create_texture failed"))?;

        // WebGL2 requires a bound VAO for drawArrays. We don't actually
        // use any attributes (the quad is built from gl_VertexID), but
        // we still need *something* bound. Keep one around for the
        // lifetime of the painter.
        let vao = gl
            .create_vertex_array()
            .ok_or_else(|| anyhow!("create_vertex_array failed"))?;
        gl.bind_vertex_array(Some(&vao));

        // Pre-bind everything that won't change again.
        gl.use_program(Some(&program));
        let u_tex = gl.get_uniform_location(&program, "u_tex");
        gl.uniform1i(u_tex.as_ref(), 0);
        gl.active_texture(Gl::TEXTURE0);
        gl.bind_texture(Gl::TEXTURE_2D, Some(&texture));
        gl.tex_parameteri(Gl::TEXTURE_2D, Gl::TEXTURE_MIN_FILTER, Gl::NEAREST as i32);
        gl.tex_parameteri(Gl::TEXTURE_2D, Gl::TEXTURE_MAG_FILTER, Gl::NEAREST as i32);
        gl.tex_parameteri(Gl::TEXTURE_2D, Gl::TEXTURE_WRAP_S, Gl::CLAMP_TO_EDGE as i32);
        gl.tex_parameteri(Gl::TEXTURE_2D, Gl::TEXTURE_WRAP_T, Gl::CLAMP_TO_EDGE as i32);

        // No depth / blending state we care about.
        gl.disable(Gl::DEPTH_TEST);
        gl.disable(Gl::BLEND);

        // Make sure the browser doesn't apply any implicit transforms
        // when we tex_sub_image_2d. Defaults are mostly sane but
        // UNPACK_COLORSPACE_CONVERSION_WEBGL is BROWSER_DEFAULT_WEBGL —
        // we want raw bytes, not sRGB-decoded.
        gl.pixel_storei(Gl::UNPACK_PREMULTIPLY_ALPHA_WEBGL, 0);
        gl.pixel_storei(Gl::UNPACK_FLIP_Y_WEBGL, 0);
        gl.pixel_storei(Gl::UNPACK_COLORSPACE_CONVERSION_WEBGL, Gl::NONE as i32);

        let mut p = GlPainter {
            gl,
            canvas,
            _program: program,
            texture,
            _vao: vao,
            width: 0,
            height: 0,
            presented: Cell::new(false),
        };
        p.resize(width, height)?;
        Ok(p)
    }

    /// Reallocate the texture and resize the canvas drawing buffer.
    /// Cheap — called once per session (and when the server reports a
    /// new resolution, which is rare).
    pub fn resize(&mut self, width: u32, height: u32) -> Result<()> {
        if width == 0 || height == 0 {
            return Ok(());
        }
        if self.canvas.width() != width {
            self.canvas.set_width(width);
        }
        if self.canvas.height() != height {
            self.canvas.set_height(height);
        }
        self.gl.viewport(0, 0, width as i32, height as i32);
        self.gl.bind_texture(Gl::TEXTURE_2D, Some(&self.texture));
        // Allocate (or reallocate) immutable RGBA8 storage. tex_storage_2d
        // is the WebGL2 way; once allocated we only ever tex_sub_image_2d
        // into it. If width/height match the previous call this would
        // fail (immutable), so we recreate the texture on resize.
        self.gl.delete_texture(Some(&self.texture));
        let new_tex = self
            .gl
            .create_texture()
            .ok_or_else(|| anyhow!("create_texture failed on resize"))?;
        self.gl.bind_texture(Gl::TEXTURE_2D, Some(&new_tex));
        self.gl
            .tex_parameteri(Gl::TEXTURE_2D, Gl::TEXTURE_MIN_FILTER, Gl::NEAREST as i32);
        self.gl
            .tex_parameteri(Gl::TEXTURE_2D, Gl::TEXTURE_MAG_FILTER, Gl::NEAREST as i32);
        self.gl
            .tex_parameteri(Gl::TEXTURE_2D, Gl::TEXTURE_WRAP_S, Gl::CLAMP_TO_EDGE as i32);
        self.gl
            .tex_parameteri(Gl::TEXTURE_2D, Gl::TEXTURE_WRAP_T, Gl::CLAMP_TO_EDGE as i32);
        self.gl
            .tex_storage_2d(Gl::TEXTURE_2D, 1, Gl::RGBA8, width as i32, height as i32);
        self.texture = new_tex;
        self.width = width;
        self.height = height;
        // New texture is empty; force the next render to draw this canvas.
        self.presented.set(false);
        Ok(())
    }

    /// Upload one `w x h` dirty sub-rectangle directly from the desktop
    /// framebuffer into the (viewport-sized) texture. Does NOT draw — `draw`
    /// issues a single fullscreen-quad pass after all of a frame's dirty
    /// rects have been uploaded.
    ///
    /// `framebuffer` is the full desktop image (`fb_width` pixels/row). The
    /// source rect at framebuffer `(src_x, src_y)` lands at texture `(dst_x,
    /// dst_y)`. `UNPACK_ROW_LENGTH`/`SKIP_PIXELS`/`SKIP_ROWS` let GL read the
    /// rect straight out of wasm memory — no scratch copy, and only the changed
    /// columns/rows are transferred (not the full row width). Texels outside the
    /// rect keep their value (`tex_sub_image_2d`, not `tex_image_2d`).
    #[expect(
        clippy::too_many_arguments,
        reason = "explicit src/dst rect avoids a wrapper struct"
    )]
    pub fn upload_rect(
        &self,
        framebuffer: &[u8],
        fb_width: u32,
        dst_x: u32,
        dst_y: u32,
        src_x: u32,
        src_y: u32,
        w: u32,
        h: u32,
    ) -> Result<()> {
        if self.width == 0 || self.height == 0 || w == 0 || h == 0 {
            return Ok(());
        }
        // The program and texture are bound once (in `new`/`resize`) and never
        // changed — this painter owns its WebGL context, so nothing else can
        // unbind them. So no per-call `use_program` / `bind_texture` is needed:
        // `tex_sub_image_2d` targets the already-bound texture.
        //
        // Read the dirty rect directly from the framebuffer: row stride =
        // fb_width, skip src_x columns and src_y rows.
        self.gl.pixel_storei(Gl::UNPACK_ROW_LENGTH, fb_width as i32);
        self.gl.pixel_storei(Gl::UNPACK_SKIP_PIXELS, src_x as i32);
        self.gl.pixel_storei(Gl::UNPACK_SKIP_ROWS, src_y as i32);
        // SAFETY: `Uint8Array::view` aliases wasm memory; it must be consumed
        // before any allocation that could relocate the backing buffer.
        // `tex_sub_image_2d` uploads synchronously here, no allocation between.
        let view = unsafe { js_sys::Uint8Array::view(framebuffer) };
        let res = self
            .gl
            .tex_sub_image_2d_with_i32_and_i32_and_u32_and_type_and_opt_array_buffer_view(
                Gl::TEXTURE_2D,
                0,
                dst_x as i32,
                dst_y as i32,
                w as i32,
                h as i32,
                Gl::RGBA,
                Gl::UNSIGNED_BYTE,
                Some(&view),
            );
        // Reset unpack state so it can't leak into any later upload.
        self.gl.pixel_storei(Gl::UNPACK_ROW_LENGTH, 0);
        self.gl.pixel_storei(Gl::UNPACK_SKIP_PIXELS, 0);
        self.gl.pixel_storei(Gl::UNPACK_SKIP_ROWS, 0);
        res.map_err(|e| anyhow!("tex_sub_image_2d failed: {e:?}"))?;
        Ok(())
    }

    /// Draw the textured fullscreen quad. Call once per frame after that
    /// frame's [`GlPainter::upload_rect`] calls. The program is already active
    /// from `new`; nothing rebinds it. The canvas always shows the current
    /// texture state — even on region-less PDUs (frame markers, cursor-only) —
    /// avoiding stripes from a post-commit clear.
    pub fn draw(&self) {
        if self.width == 0 || self.height == 0 {
            return;
        }
        self.gl.draw_arrays(Gl::TRIANGLES, 0, 6);
        self.presented.set(true);
    }

    /// Clear the drawing buffer to opaque black and reset the texture to
    /// empty. Used on session reset. Replaces the canvas2D `clearRect`.
    pub fn clear(&self) {
        if self.width == 0 || self.height == 0 {
            return;
        }
        self.gl.clear_color(0.0, 0.0, 0.0, 1.0);
        self.gl.clear(Gl::COLOR_BUFFER_BIT);
        self.presented.set(false);
    }

    /// Whether the quad has been drawn at least once since the last (re)size.
    /// A canvas that uploaded nothing this frame but has already presented
    /// keeps its image via `preserveDrawingBuffer=true` and needn't redraw —
    /// saving a `drawArrays` per idle canvas per frame (scales with monitor
    /// count). Consumed by a multi-monitor fan-out layer; unused on the
    /// single-canvas FastPath path.
    #[allow(dead_code)]
    pub fn presented(&self) -> bool {
        self.presented.get()
    }
}

fn compile_shader(gl: &Gl, kind: u32, src: &str) -> Result<WebGlShader> {
    let shader = gl
        .create_shader(kind)
        .ok_or_else(|| anyhow!("create_shader({kind}) failed"))?;
    gl.shader_source(&shader, src);
    gl.compile_shader(&shader);
    let ok = gl
        .get_shader_parameter(&shader, Gl::COMPILE_STATUS)
        .as_bool()
        .unwrap_or(false);
    if !ok {
        let log = gl
            .get_shader_info_log(&shader)
            .unwrap_or_else(|| "<no log>".into());
        return Err(anyhow!("shader compile failed: {log}"));
    }
    Ok(shader)
}

fn build_program(gl: &Gl) -> Result<WebGlProgram> {
    let vs = compile_shader(gl, Gl::VERTEX_SHADER, VERT_SRC)?;
    let fs = compile_shader(gl, Gl::FRAGMENT_SHADER, FRAG_SRC)?;
    let program = gl
        .create_program()
        .ok_or_else(|| anyhow!("create_program failed"))?;
    gl.attach_shader(&program, &vs);
    gl.attach_shader(&program, &fs);
    gl.link_program(&program);
    let ok = gl
        .get_program_parameter(&program, Gl::LINK_STATUS)
        .as_bool()
        .unwrap_or(false);
    if !ok {
        let log = gl
            .get_program_info_log(&program)
            .unwrap_or_else(|| "<no log>".into());
        return Err(anyhow!("program link failed: {log}"));
    }
    // Shaders are kept attached but can be deleted now — the program
    // retains them internally.
    gl.delete_shader(Some(&vs));
    gl.delete_shader(Some(&fs));
    Ok(program)
}
