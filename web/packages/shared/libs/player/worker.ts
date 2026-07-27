/**
 * Dedicated worker for the single-monitor harness (`DesktopSessionTest.tsx`).
 * The multi-monitor variant runs the decoder inside the SharedWorker; this
 * file is only used when there's one canvas and one window.
 */

import init, {
  initWasmLog,
  WorkerDecoder,
  type WorkerDecoder as WorkerDecoderType,
} from './pkg/session';
import {
  IncomingMessageType,
  OutgoingMessageType,
  type IncomingMessage,
  type OutgoingMessage,
} from './types';

declare const self: Omit<typeof globalThis, 'postMessage'> & {
  postMessage: (message: OutgoingMessage) => void;
  addEventListener: typeof addEventListener;
};

let decoder: WorkerDecoderType | null = null;
let wasmReady: Promise<unknown> | null = null;

self.addEventListener(
  'message',
  async (event: MessageEvent<IncomingMessage>) => {
    const msg = event.data;
    switch (msg.type) {
      case IncomingMessageType.Init: {
        if (!wasmReady) {
          wasmReady = init().then(() => initWasmLog('debug'));
        }
        await wasmReady;
        // Decoder events come back through this callback. We forward them
        // straight onto the worker's postMessage so the main thread can pick
        // them up via the OutgoingMessage union.
        decoder = new WorkerDecoder((m: OutgoingMessage) => {
          self.postMessage(m);
        });
        // Register the canvas with a fixed id (0) — single-monitor mode only
        // ever has one. If no viewport is specified, use sentinel max-values
        // so the framebuffer clamps the viewport to the actual image bounds
        // once dims are known (after ServerHello).
        const vp = msg.viewport ?? {
          x: 0,
          y: 0,
          width: 0xffffffff,
          height: 0xffffffff,
        };
        decoder.addCanvas(0, msg.canvas, vp.x, vp.y, vp.width, vp.height);
        self.postMessage({ type: OutgoingMessageType.Ready });
        break;
      }
      case IncomingMessageType.Bytes:
        decoder?.feedBytes(msg.buffer);
        break;
      case IncomingMessageType.Close:
        decoder = null;
        break;
    }
  }
);
