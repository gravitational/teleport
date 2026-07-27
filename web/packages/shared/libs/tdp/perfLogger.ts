/**
 * Periodic per-stage timing for the live production RDP client. Field
 * naming is kept identical to `shared/libs/player/perfLogger.ts` so a
 * `console` grep on `[perf-` can pivot between `[perf-old]` and `[perf-new]`
 * line by line and read off matching columns.
 *
 * The old client runs entirely on the main thread, so this module is just
 * a singleton accumulator that `client.ts` (decode + ironrdp.process +
 * response/mouse send) and `CanvasRenderer.tsx` (put_image, cursor) update
 * from their hot paths. A `setInterval` at 1Hz flushes the line and
 * resets.
 *
 * pixel_copy is reported as 0/0/0 because the old client's pixel copy
 * happens *inside* the ironrdp wasm module's `process()` call (it
 * allocates a per-region Vec<u8> and an ImageData), so it's bundled into
 * `ironrdp_process`. Worth keeping in mind when comparing to the new
 * client where the copy is broken out.
 */

interface Stage {
  count: number;
  sumMs: number;
  maxMs: number;
}

function newStage(): Stage {
  return { count: 0, sumMs: 0, maxMs: 0 };
}

function record(stage: Stage, ms: number): void {
  stage.count += 1;
  stage.sumMs += ms;
  if (ms > stage.maxMs) stage.maxMs = ms;
}

function fmt(ms: number): string {
  return ms.toFixed(2);
}

function mean(stage: Stage): number {
  return stage.count === 0 ? 0 : stage.sumMs / stage.count;
}

class TdpPerf {
  private codecDecode = newStage();
  private ironrdpProcess = newStage();
  private putImage = newStage();
  private responseSend = newStage();
  private cursorPost = newStage();
  private mouseEncode = newStage();
  private mouseSend = newStage();

  private pdus = 0;
  private paints = 0;
  private mouseCount = 0;
  private dirtyPixels = 0;
  private responseBytes = 0;
  private domMouseMoveCount = 0;

  private rafSamples: number[] = [];
  private lastRafMs = 0;
  private rafId: number | null = null;

  private flushTimer: ReturnType<typeof setInterval> | null = null;

  private now(): number {
    // performance.now is available in both window and worker contexts; the
    // old client only runs in the main thread.
    return typeof performance !== 'undefined' ? performance.now() : Date.now();
  }

  recordDecode(ms: number) {
    record(this.codecDecode, ms);
  }

  recordProcess(ms: number) {
    record(this.ironrdpProcess, ms);
    this.pdus += 1;
  }

  recordPutImage(ms: number, dirtyW: number, dirtyH: number) {
    record(this.putImage, ms);
    this.paints += 1;
    this.dirtyPixels += dirtyW * dirtyH;
  }

  recordResponseSend(ms: number, bytes: number) {
    record(this.responseSend, ms);
    this.responseBytes += bytes;
  }

  recordCursorPost(ms: number) {
    record(this.cursorPost, ms);
  }

  recordMouseEncode(ms: number) {
    record(this.mouseEncode, ms);
    this.mouseCount += 1;
  }

  recordMouseSend(ms: number) {
    record(this.mouseSend, ms);
  }

  recordDomMouseMove() {
    this.domMouseMoveCount += 1;
  }

  /** Run an arbitrary block and record the elapsed time into `stage`. */
  time<T>(stage: (ms: number) => void, fn: () => T): T {
    const start = this.now();
    try {
      return fn();
    } finally {
      stage.call(this, this.now() - start);
    }
  }

  start() {
    if (this.flushTimer !== null) return;
    const tick = (ts: number) => {
      if (this.lastRafMs !== 0) this.rafSamples.push(ts - this.lastRafMs);
      this.lastRafMs = ts;
      this.rafId = requestAnimationFrame(tick);
    };
    this.rafId = requestAnimationFrame(tick);
    this.flushTimer = setInterval(() => this.flush(), 1000);
  }

  stop() {
    if (this.flushTimer !== null) {
      clearInterval(this.flushTimer);
      this.flushTimer = null;
    }
    if (this.rafId !== null) {
      cancelAnimationFrame(this.rafId);
      this.rafId = null;
    }
  }

  private flush() {
    const elapsedMs = 1000; // setInterval cadence; matches the new client's
    if (this.pdus === 0 && this.mouseCount === 0 && this.paints === 0) {
      // Idle window — skip to keep console quiet, mirroring the Rust side
      // which only flushes when a frame or mouse event hit the timer.
      this.resetCounters();
      return;
    }
    const sec = elapsedMs / 1000;
    const avgDirty = this.paints > 0 ? Math.round(this.dirtyPixels / this.paints) : 0;
    let rafMean = 0;
    let rafMax = 0;
    if (this.rafSamples.length > 0) {
      let s = 0;
      for (const v of this.rafSamples) {
        s += v;
        if (v > rafMax) rafMax = v;
      }
      rafMean = s / this.rafSamples.length;
    }
    // eslint-disable-next-line no-console
    console.log(
      `[perf-old] ${sec.toFixed(2)}s ` +
        `pdus/s=${(this.pdus / sec).toFixed(1)} ` +
        `paints/s=${(this.paints / sec).toFixed(1)} ` +
        `mouse_out/s=${(this.mouseCount / sec).toFixed(1)} ` +
        `avg_dirty_px=${avgDirty} resp_bytes=${this.responseBytes} | ` +
        `decode=${fmt(mean(this.codecDecode))}/${fmt(this.codecDecode.maxMs)}ms ` +
        `process=${fmt(mean(this.ironrdpProcess))}/${fmt(this.ironrdpProcess.maxMs)}ms ` +
        `pix_copy=0.00/0.00ms ` + // bundled into ironrdp_process; see header comment
        `put_img=${fmt(mean(this.putImage))}/${fmt(this.putImage.maxMs)}ms ` +
        `resp_send=${fmt(mean(this.responseSend))}/${fmt(this.responseSend.maxMs)}ms ` +
        `cursor=${fmt(mean(this.cursorPost))}/${fmt(this.cursorPost.maxMs)}ms ` +
        `m_enc=${fmt(mean(this.mouseEncode))}/${fmt(this.mouseEncode.maxMs)}ms ` +
        `m_send=${fmt(mean(this.mouseSend))}/${fmt(this.mouseSend.maxMs)}ms | ` +
        `dom_mouse/s=${(this.domMouseMoveCount / sec).toFixed(1)} ` +
        `raf=${fmt(rafMean)}/${fmt(rafMax)}ms`
    );
    this.resetCounters();
  }

  private resetCounters() {
    this.codecDecode = newStage();
    this.ironrdpProcess = newStage();
    this.putImage = newStage();
    this.responseSend = newStage();
    this.cursorPost = newStage();
    this.mouseEncode = newStage();
    this.mouseSend = newStage();
    this.pdus = 0;
    this.paints = 0;
    this.mouseCount = 0;
    this.dirtyPixels = 0;
    this.responseBytes = 0;
    this.domMouseMoveCount = 0;
    this.rafSamples = [];
  }

  perfNow(): number {
    return this.now();
  }
}

/**
 * Singleton — both `client.ts` and `CanvasRenderer.tsx` import this same
 * instance. The first call to `.start()` (made from the client when its
 * transport opens) arms the 1Hz flush; `.stop()` is called on disconnect.
 */
export const TDP_PERF = new TdpPerf();
