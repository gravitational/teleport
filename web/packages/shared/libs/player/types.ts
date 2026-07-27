/**
 * Messages exchanged between the main thread and the `worker.ts` session
 * worker. Field shapes here match what the Rust `WorkerDecoder` posts via
 * `Reflect::set` in `crates/session/src/lib.rs`, so the two must move in
 * lockstep.
 *
 * Post-A.2 architecture: main thread owns the WebSocket and the outbound
 * codec; worker only decodes inbound + paints the OffscreenCanvas + posts
 * back the things main needs to do something with.
 */

export enum IncomingMessageType {
  /** Hand the worker its OffscreenCanvas. Sent once at startup. */
  Init = 'init',
  /** Raw WS frame for the worker to decode. Buffer is transferred (zero copy). */
  Bytes = 'bytes',
  /** Tear down before main goes away. */
  Close = 'close',
}

export enum OutgoingMessageType {
  /** Worker finished `init` and is ready to receive `bytes`. */
  Ready = 'ready',
  Log = 'log',
  Decoded = 'decoded',
  /** Server-confirmed screen resolution; main uses it to scale mouse coords. */
  Resolution = 'resolution',
  /** Server-supplied cursor bitmap to apply as `canvas.style.cursor`. */
  CursorBitmap = 'cursorBitmap',
  /** Server hid the cursor (`cursor: none`). */
  CursorHidden = 'cursorHidden',
  /** Server requested the default cursor (`cursor: default`). */
  CursorDefault = 'cursorDefault',
  /**
   * Server has switched to TDPB. Main flips its own codec mode and sends
   * the ClientHello — main owns the WS, so the send happens there.
   */
  TdpbUpgrade = 'tdpbUpgrade',
  /**
   * IronRDP wants these raw bytes shipped back to the server. Main
   * wraps them as an `RdpResponsePdu` envelope (via `MainCodec`) and
   * sends. The buffer is transferred zero-copy.
   */
  RdpResponse = 'rdpResponse',
  /** Periodic performance stats from the worker (~1Hz). */
  Perf = 'perf',
  /** One-shot notification when a single FastPath PDU exceeds the slow threshold. */
  SlowPdu = 'slowPdu',
}

export type IncomingMessage =
  | {
      type: IncomingMessageType.Init;
      canvas: OffscreenCanvas;
      /**
       * Multi-monitor mode: this window should render only the given slice
       * of the virtual desktop. Coordinates are in virtual-desktop pixels;
       * the canvas drawing buffer is sized to `viewport.width × height`.
       * Omit for single-monitor sessions.
       */
      viewport?: { x: number; y: number; width: number; height: number };
    }
  | { type: IncomingMessageType.Bytes; buffer: ArrayBuffer }
  | { type: IncomingMessageType.Close };

/**
 * Per-stage timing snapshot the worker emits every ~1s. Field naming
 * mirrors `crates/session/src/perf.rs`; the old client's `[perf-old]`
 * line uses the same names for direct comparison.
 *
 * Outbound stages (mouse encode/send) live on main now and aren't part
 * of this payload — main mixes them in when formatting the line.
 */
export interface PerfPayload {
  type: OutgoingMessageType.Perf;
  elapsed_ms: number;
  pdus: number;
  paints: number;
  dirty_pixels: number;
  response_bytes: number;
  codec_decode_n: number;
  codec_decode_mean_ms: number;
  codec_decode_max_ms: number;
  ironrdp_process_n: number;
  ironrdp_process_mean_ms: number;
  ironrdp_process_max_ms: number;
  process_surface_n: number;
  process_surface_mean_ms: number;
  process_surface_max_ms: number;
  process_bitmap_n: number;
  process_bitmap_mean_ms: number;
  process_bitmap_max_ms: number;
  process_pointer_n: number;
  process_pointer_mean_ms: number;
  process_pointer_max_ms: number;
  process_other_n: number;
  process_other_mean_ms: number;
  process_other_max_ms: number;
  pixel_copy_n: number;
  pixel_copy_mean_ms: number;
  pixel_copy_max_ms: number;
  put_image_n: number;
  put_image_mean_ms: number;
  put_image_max_ms: number;
  response_send_n: number;
  response_send_mean_ms: number;
  response_send_max_ms: number;
  cursor_post_n: number;
  cursor_post_mean_ms: number;
  cursor_post_max_ms: number;
}

export type OutgoingMessage =
  | { type: OutgoingMessageType.Ready }
  | { type: OutgoingMessageType.Log; text: string }
  | { type: OutgoingMessageType.Decoded; text: string }
  | { type: OutgoingMessageType.Resolution; width: number; height: number }
  | {
      type: OutgoingMessageType.CursorBitmap;
      imageData: ImageData;
      width: number;
      height: number;
      hotspotX: number;
      hotspotY: number;
    }
  | { type: OutgoingMessageType.CursorHidden }
  | { type: OutgoingMessageType.CursorDefault }
  | { type: OutgoingMessageType.TdpbUpgrade }
  | { type: OutgoingMessageType.RdpResponse; buffer: ArrayBuffer }
  | PerfPayload
  | {
      type: OutgoingMessageType.SlowPdu;
      class: 'surface' | 'bitmap' | 'pointer' | 'other';
      ms: number;
      len: number;
    };
