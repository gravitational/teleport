/* eslint-disable no-console, @typescript-eslint/no-explicit-any */
/**
 * In-page log sink + reducer toolkit for the codec-test harness.
 *
 * Problem it solves: the EGFX/black-rects debugging probes emit thousands of
 * near-identical `console.warn` lines (mostly from the wasm `framebuffer.rs`
 * running in the worker). Copying them out of DevTools is huge and the browser
 * decorates every line with a `file:line` source anchor. This module captures
 * the RAW log strings (no source anchor) into a capped ring buffer and lets you
 * reduce them to a tiny, paste-able answer on demand.
 *
 * NON-DESTRUCTIVE: every line is stored. Reducers (`distinct`/`grep`/`tail`/…)
 * and `filter` are read-time VIEWS over the full buffer — they never drop a
 * captured line. So a single repro retains everything and can be drilled into
 * progressively (alerts -> grep the slot -> tail for context) with NO re-run.
 * The only thing that crosses to you is the small reduced result.
 *
 * Two realms:
 *   - The worker (`codecTestWorker.ts`) is where the wasm `console::warn` probes
 *     land. It installs the sink + detectors and answers on-demand reduction
 *     queries from main (`__logQuery` -> `__logResult`).
 *   - Main (`DesktopSessionTestMulti.tsx`) installs a thin async `globalThis.L`
 *     that reduces its own buffer AND forwards the same query to the worker,
 *     merging the two small results — so you can drive everything from the main
 *     DevTools console.
 *
 * Per-line traffic across realms is ZERO: each realm buffers locally and only
 * the (already reduced) result crosses on demand.
 *
 * Console usage (main page console):
 *   copy(await L.distinct('[c2s-black]'))   // deduped lines, sorted by count
 *   copy(await L.alerts())                  // detector hits (the bug firing)
 *   copy(await L.summary())                 // per-tag counts (always whole buffer)
 *   await L.filter('[pixhist]')             // scope the VIEW to one probe (non-destructive)
 *   copy(await L.grep(/slot=137/))          // drill into a specific slot, same repro
 *   copy(await L.tail(80))                  // recent context around an event
 *   await L.clear()                         // reset before a fresh repro
 * `copy(...)` is the reliable grab; the methods also best-effort write the
 * clipboard directly (which needs page focus).
 */

type Matcher = string | RegExp | undefined;

// --- buffers -------------------------------------------------------------
// Generous cap: we store EVERY line so nothing is lost mid-investigation. At
// ~100 bytes/line this is ~50MB worst case, fine for a debug session. `clear()`
// before each repro keeps a run well under this.
const RING_CAP = 500_000;
const ALERT_CAP = 5_000;

interface SinkState {
  ring: string[];
  alerts: string[];
  patched: boolean;
  inIngest: boolean;
  lastPix: { r: number; g: number; b: number; a: number } | null;
  origConsole: {
    log: (...a: unknown[]) => void;
    warn: (...a: unknown[]) => void;
    error: (...a: unknown[]) => void;
  } | null;
}

// State lives on globalThis (not module scope) so it survives Vite HMR: a
// re-imported module instance shares the SAME ring + patch flag instead of
// double-patching console and reading an empty buffer.
const S: SinkState = ((globalThis as any).__logsink ??= {
  ring: [],
  alerts: [],
  patched: false,
  inIngest: false,
  lastPix: null,
  origConsole: null,
});

// --- capture -------------------------------------------------------------
function fmt(args: unknown[]): string {
  return args
    .map(a => {
      if (typeof a === 'string') return a;
      if (a instanceof Error) return a.stack ? a.stack.split('\n')[0] : a.message;
      if (a === null) return 'null';
      if (a === undefined) return 'undefined';
      try {
        return JSON.stringify(a);
      } catch {
        return String(a);
      }
    })
    .join(' ');
}

function ingest(text: string): void {
  // Non-destructive: EVERY line is detected and stored. Filtering happens at
  // read time (see effective()), never here — so we can never lose a line we
  // might want later in the same repro.
  runDetectors(text);
  S.ring.push(text);
  if (S.ring.length > RING_CAP) S.ring.splice(0, S.ring.length - RING_CAP);
}

function patchConsole(): void {
  if (S.patched) return;
  S.patched = true;
  const c = (globalThis as any).console;
  S.origConsole = {
    log: c.log.bind(c),
    warn: c.warn.bind(c),
    error: c.error.bind(c),
  };
  // Capture the full set of levels: the wasm `debug`-level logging may route to
  // console.debug/info rather than console.warn.
  for (const level of ['log', 'info', 'warn', 'error', 'debug'] as const) {
    if (typeof c[level] !== 'function') continue;
    const orig = c[level].bind(c);
    c[level] = (...args: unknown[]) => {
      orig(...args);
      // Re-entrancy guard: a detector's live [ALERT] uses the ORIGINAL console
      // (see pushAlert), but guard anyway so nothing can loop through ingest.
      if (S.inIngest) return;
      S.inIngest = true;
      try {
        ingest(fmt(args));
      } catch {
        /* never let logging break */
      }
      S.inIngest = false;
    };
  }
}

// --- detectors -----------------------------------------------------------
function pushAlert(a: string): void {
  S.alerts.push(a);
  if (S.alerts.length > ALERT_CAP) S.alerts.splice(0, S.alerts.length - ALERT_CAP);
  // Live one-liner via the ORIGINAL console so it's visible as it happens and
  // does not re-enter ingest.
  S.origConsole?.warn(`[ALERT] ${a}`);
}

function runDetectors(text: string): void {
  // Detector 1 — pixel goes black. Watches the Detection-A `[pixhist]` probe
  // (`op=<op> ... -> (r,g,b,a)`). Fires on a non-black -> opaque-black
  // transition, naming the op (and cache slot, if present) that did it.
  const pm = text.match(/\[pixhist\]\s+op=(\S+)/);
  if (pm) {
    const rgba = text.match(/->\s*\((\d+),\s*(\d+),\s*(\d+),\s*(\d+)\)/);
    if (rgba) {
      const r = +rgba[1];
      const g = +rgba[2];
      const b = +rgba[3];
      const a = +rgba[4];
      const isBlack = r === 0 && g === 0 && b === 0 && a === 255;
      const prev = S.lastPix;
      const prevBlack =
        prev && prev.r === 0 && prev.g === 0 && prev.b === 0 && prev.a === 255;
      if (isBlack && prev && !prevBlack) {
        const slot = text.match(/slot=(\d+)/)?.[1];
        pushAlert(
          `pixel-blacked op=${pm[1]}${slot ? ` slot=${slot}` : ''} was=(${prev.r},${prev.g},${prev.b},${prev.a})`
        );
      }
      S.lastPix = { r, g, b, a };
    }
    return;
  }

  // Detector 2 — a black bitmap-cache slot is snapshotted/replayed. Fires now
  // on the existing `[c2s-black]` / `[cache-snap-black]` probes.
  if (text.includes('[c2s-black]') || text.includes('[cache-snap-black]')) {
    const black = text.match(/black=(\d+)/)?.[1];
    if (black) {
      const slot = text.match(/slot=(\d+)/)?.[1] ?? '?';
      pushAlert(`cacheslot-black slot=${slot} black=${black}%`);
    }
  }
}

// --- reducers (read-time VIEWS over the full ring, return a compact string) --
function toTest(m: Matcher): (s: string) => boolean {
  if (m == null) return () => true;
  if (m instanceof RegExp) return s => m.test(s);
  return s => s.includes(m);
}

/** Read-time default view filter set via L.filter(). Used by reducers when no
 * explicit matcher is passed. NEVER drops lines from the ring. */
function defaultMatcher(): Matcher {
  const f = (globalThis as any).__logFilter;
  return typeof f === 'string' && f.length ? f : undefined;
}

function effective(m: Matcher): Matcher {
  return m !== undefined ? m : defaultMatcher();
}

function rDistinct(m: Matcher): string {
  const eff = effective(m);
  const test = toTest(eff);
  const counts = new Map<string, number>();
  for (const line of S.ring) if (test(line)) counts.set(line, (counts.get(line) ?? 0) + 1);
  const sorted = [...counts.entries()].sort((a, b) => b[1] - a[1]);
  const total = sorted.reduce((s, [, c]) => s + c, 0);
  const LIM = 200;
  let head = `# distinct: ${sorted.length} unique / ${total} matched (${S.ring.length} stored)${eff ? ` filter=${String(eff)}` : ''}`;
  if (sorted.length > LIM) head += ` (top ${LIM})`;
  const body = sorted
    .slice(0, LIM)
    .map(([l, c]) => `${String(c).padStart(6)} × ${l}`)
    .join('\n');
  return body ? `${head}\n${body}` : head;
}

function rGrep(m: Matcher): string {
  const eff = effective(m);
  const hits = S.ring.filter(toTest(eff));
  const LIM = 500;
  let head = `# grep: ${hits.length} hits (${S.ring.length} stored)${eff ? ` ${String(eff)}` : ''}`;
  if (hits.length > LIM) head += ` (last ${LIM})`;
  const body = hits.slice(-LIM).join('\n');
  return body ? `${head}\n${body}` : head;
}

function rTail(n: number): string {
  const eff = defaultMatcher();
  const view = eff ? S.ring.filter(toTest(eff)) : S.ring;
  const body = view.slice(-n).join('\n');
  return body
    ? `# tail ${n} of ${view.length}${eff ? ` filter=${String(eff)}` : ''}\n${body}`
    : `# tail: empty`;
}

function rHead(n: number): string {
  const eff = defaultMatcher();
  const view = eff ? S.ring.filter(toTest(eff)) : S.ring;
  const body = view.slice(0, n).join('\n');
  return body
    ? `# head ${n} of ${view.length}${eff ? ` filter=${String(eff)}` : ''}\n${body}`
    : `# head: empty`;
}

function rSummary(): string {
  // Always the WHOLE buffer — this is the "what's flowing" overview, never
  // scoped by the default filter.
  const tags = new Map<string, number>();
  for (const line of S.ring) {
    const t = line.match(/^\s*(\[[^\]]+\])/)?.[1] ?? '(untagged)';
    tags.set(t, (tags.get(t) ?? 0) + 1);
  }
  const sorted = [...tags.entries()].sort((a, b) => b[1] - a[1]);
  const body = sorted.map(([t, c]) => `${String(c).padStart(6)} ${t}`).join('\n');
  const f = defaultMatcher();
  return `# summary: ${S.ring.length} lines, ${tags.size} tags, ${S.alerts.length} alerts${f ? ` (view filter=${String(f)})` : ''}\n${body}`;
}

function rAlerts(): string {
  // Detector hits — always complete, independent of the view filter.
  const counts = new Map<string, number>();
  for (const a of S.alerts) counts.set(a, (counts.get(a) ?? 0) + 1);
  const sorted = [...counts.entries()].sort((a, b) => b[1] - a[1]);
  const body = sorted.map(([a, c]) => `${String(c).padStart(6)} × ${a}`).join('\n');
  return `# alerts: ${S.alerts.length} total, ${sorted.length} distinct\n${body}`;
}

function clearAll(): string {
  S.ring.length = 0;
  S.alerts.length = 0;
  S.lastPix = null;
  return '# cleared';
}

// --- cross-realm reducer dispatch ---------------------------------------
function serializeMatcher(m: Matcher): any {
  if (m instanceof RegExp) return { __re: m.source, flags: m.flags };
  return m;
}

function deserializeMatcher(x: any): Matcher {
  if (x && typeof x === 'object' && typeof x.__re === 'string') {
    return new RegExp(x.__re, x.flags ?? '');
  }
  return x;
}

// [perf-probe] On-demand perf snapshot provider, registered by the worker
// (codecTestWorker) so `L.perf()` reports live AVC decoder stats — GPU vs CPU
// present path, decode/output gap, decoder queue depths, JS heap — without
// periodic console spam. Null in realms that don't register one (e.g. main).
let perfProvider: (() => string) | null = null;
export function setLogPerfProvider(fn: () => string): void {
  perfProvider = fn;
}

/** Run a reduction over THIS realm's buffer. `arg` is a serialized matcher for
 * distinct/grep, a number for tail/head, a string for filter. */
function reduceLocal(op: string, arg: any): string {
  switch (op) {
    case 'distinct':
      return rDistinct(deserializeMatcher(arg));
    case 'grep':
      return rGrep(deserializeMatcher(arg));
    case 'tail':
      return rTail(typeof arg === 'number' ? arg : 200);
    case 'head':
      return rHead(typeof arg === 'number' ? arg : 200);
    case 'summary':
      return rSummary();
    case 'alerts':
      return rAlerts();
    case 'clear':
      return clearAll();
    case 'filter':
      (globalThis as any).__logFilter = typeof arg === 'string' ? arg : '';
      return `# filter=${(globalThis as any).__logFilter || '(none)'}`;
    case 'perf':
      return perfProvider ? perfProvider() : '# no perf provider';
    default:
      return `# unknown op ${op}`;
  }
}

async function copyClip(text: string): Promise<void> {
  try {
    const nav = (globalThis as any).navigator;
    if (nav?.clipboard?.writeText) await nav.clipboard.writeText(text);
  } catch {
    /* clipboard write needs page focus; `copy(await L.x())` is the fallback */
  }
}

// --- worker realm --------------------------------------------------------
/** Local (synchronous) `L`, for when the DevTools console context is set to the
 * worker. Each method copies its small result and returns it. */
function makeLocalL() {
  return {
    distinct: (m?: Matcher) => {
      const o = rDistinct(m);
      void copyClip(o);
      return o;
    },
    grep: (m?: Matcher) => {
      const o = rGrep(m);
      void copyClip(o);
      return o;
    },
    tail: (n = 200) => {
      const o = rTail(n);
      void copyClip(o);
      return o;
    },
    head: (n = 200) => {
      const o = rHead(n);
      void copyClip(o);
      return o;
    },
    summary: () => {
      const o = rSummary();
      void copyClip(o);
      return o;
    },
    alerts: () => {
      const o = rAlerts();
      void copyClip(o);
      return o;
    },
    clear: () => clearAll(),
    perf: () => {
      const o = perfProvider ? perfProvider() : '# no perf provider';
      void copyClip(o);
      return o;
    },
    filter: (t = '') => {
      (globalThis as any).__logFilter = t;
      return `# filter=${t || '(none)'}`;
    },
    // Run several queries, join their results, copy the lot in one shot.
    // e.g. L.cap(L.distinct('[present]'), L.grep('[present] (1600,330)'))
    cap: (...qs: any[]) =>
      Promise.all(qs).then(a => {
        const out = a.join('\n\n---\n\n');
        void copyClip(out);
        return out;
      }),
  };
}

export function installWorkerLogSink(): void {
  patchConsole();
  (globalThis as any).L = makeLocalL();
  // Answer reduction queries posted by main. Registered as an additional
  // listener alongside the worker's own `message` handler; both fire, and each
  // ignores the other's message shapes.
  (globalThis as any).addEventListener('message', (e: any) => {
    const m = e?.data;
    if (!m || m.__logQuery !== true) return;
    let text: string;
    try {
      text = reduceLocal(m.op, m.arg);
    } catch (err) {
      text = `# worker reducer error: ${String(err)}`;
    }
    (globalThis as any).postMessage({ __logResult: m.id, text });
  });
  S.origConsole?.log(
    '[logsink] worker sink ready (L.* available in worker console context)'
  );
}

// --- main realm ----------------------------------------------------------
let theWorker: { postMessage: (m: any) => void } | null = null;
const pending = new Map<number, (t: string) => void>();
let queryId = 0;

function queryWorker(op: string, arg: any): Promise<string> {
  if (!theWorker) return Promise.resolve('');
  const id = ++queryId;
  return new Promise<string>(resolve => {
    const timer = setTimeout(() => {
      pending.delete(id);
      resolve('# (worker: no reply)');
    }, 1500);
    pending.set(id, t => {
      clearTimeout(timer);
      resolve(t);
    });
    theWorker!.postMessage({ __logQuery: true, id, op, arg });
  });
}

function bodyEmpty(s: string): boolean {
  const nl = s.indexOf('\n');
  return nl < 0 || s.slice(nl + 1).trim().length === 0;
}

function mergeRealms(local: string, remote: string): string {
  const parts: string[] = [];
  if (local && !bodyEmpty(local)) parts.push(`## main\n${local}`);
  if (remote && !bodyEmpty(remote)) parts.push(`## worker\n${remote}`);
  if (parts.length === 0) return remote || local;
  return parts.join('\n\n');
}

/** Async `L` for the main console: reduces main's buffer + the worker's, merges
 * the two small results, copies, and returns. */
function makeAsyncL() {
  const run = async (op: string, rawArg?: any) => {
    const arg =
      op === 'distinct' || op === 'grep' ? serializeMatcher(rawArg) : rawArg;
    const local = reduceLocal(op, arg);
    const remote = theWorker ? await queryWorker(op, arg) : '';
    const out = mergeRealms(local, remote);
    void copyClip(out);
    return out;
  };
  return {
    distinct: (m?: Matcher) => run('distinct', m),
    grep: (m?: Matcher) => run('grep', m),
    tail: (n = 200) => run('tail', n),
    head: (n = 200) => run('head', n),
    summary: () => run('summary'),
    alerts: () => run('alerts'),
    perf: () => run('perf'),
    clear: () => {
      reduceLocal('clear', null);
      theWorker?.postMessage({ __logQuery: true, id: ++queryId, op: 'clear' });
      return Promise.resolve('# cleared (both realms)');
    },
    filter: (t = '') => {
      (globalThis as any).__logFilter = t;
      theWorker?.postMessage({
        __logQuery: true,
        id: ++queryId,
        op: 'filter',
        arg: t,
      });
      return Promise.resolve(`# filter=${t || '(none)'} (view only, both realms)`);
    },
    // Run several queries, join their results, copy the lot in one shot.
    // e.g. L.cap(L.distinct('[present]'), L.grep('[present] (1600,330)'))
    cap: (...qs: any[]) =>
      Promise.all(qs).then(a => {
        const out = a.join('\n\n---\n\n');
        void copyClip(out);
        return out;
      }),
  };
}

export function installMainLogSink(): void {
  patchConsole();
  (globalThis as any).L = makeAsyncL();
  S.origConsole?.log(
    '[logsink] main sink ready (full log retained; filter is a view). Use: ' +
      'copy(await L.distinct("[c2s-black]")), copy(await L.alerts()), copy(await L.summary()), ' +
      'copy(await L.grep(/slot=137/)), await L.filter("[pixhist]"), await L.clear()'
  );
}

/** Wire main's `L` to the worker so queries reach the worker buffer. Adds a
 * `message` listener for `__logResult` alongside the existing `worker.onmessage`
 * (both fire). Call after the worker is created. */
export function attachLogSinkWorker(w: {
  postMessage: (m: any) => void;
  addEventListener: (type: 'message', cb: (e: any) => void) => void;
}): void {
  theWorker = w;
  w.addEventListener('message', (e: any) => {
    const m = e?.data;
    if (m && typeof m.__logResult === 'number') {
      const cb = pending.get(m.__logResult);
      if (cb) {
        pending.delete(m.__logResult);
        cb(m.text);
      }
    }
  });
}
