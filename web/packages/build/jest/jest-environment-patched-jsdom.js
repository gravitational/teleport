/* oxlint-disable constructor-super */
import { TransformStream, WritableStream } from 'node:stream/web';
import { TextDecoder, TextEncoder } from 'node:util';
import { BroadcastChannel } from 'node:worker_threads';

import { TestEnvironment as JSDOMEnvironment } from 'jest-environment-jsdom';

// When using jest-environment-jsdom, TextEncoder and TextDecoder are not defined. This poses a
// problem when writing tests for code which uses TextEncoder and TextDecoder directly or that
// imports libraries which depend on those globals.
//
// It's unclear if that's a problem with Jest or JSDOM itself, see
// https://github.com/jsdom/jsdom/issues/2524#issuecomment-902027138
//
// In the meantime, we're just going to "polyfill" those classes from Node ourselves.
//
// PatchedJSDOMEnvironment is taken from:
// https://github.com/jsdom/jsdom/issues/2524#issuecomment-1480930523
// https://github.com/jsdom/jsdom/issues/2524#issuecomment-1542294847
export default class PatchedJSDOMEnvironment extends JSDOMEnvironment {
  constructor(...args) {
    const { global } = super(...args);
    if (!global.TextEncoder) global.TextEncoder = TextEncoder;
    if (!global.TextDecoder) global.TextDecoder = TextDecoder;

    // TODO(sshah): Remove this once JSDOM provides structuredClone.
    // https://github.com/jsdom/jsdom/issues/3363
    if (!global.structuredClone) {
      global.structuredClone = structuredClone;
    }

    // TODO(gzdunek): Remove this once JSDOM provides scrollIntoView.
    // https://github.com/jsdom/jsdom/issues/1695#issuecomment-449931788
    if (!global.Element.prototype.scrollIntoView) {
      global.Element.prototype.scrollIntoView = () => {};
    }

    // TODO(ryanclark): Remove this once JSDOM provides scrollTo.
    // https://github.com/jsdom/jsdom/issues/2751
    if (!global.Element.prototype.scrollTo) {
      global.Element.prototype.scrollTo = () => {};
    }

    // TODO(gzdunek): Remove this once JSDOM provides matchMedia.
    // https://github.com/jsdom/jsdom/issues/3522
    if (!global.matchMedia) {
      global.matchMedia = query => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: () => {},
        removeListener: () => {},
        addEventListener: () => {},
        removeEventListener: () => {},
        dispatchEvent: () => {},
      });
    }

    // TODO(ravicious): JSDOM doesn't have BroadcastChannel and TransformStream which are used by msw.
    // https://github.com/mswjs/msw/issues/2340
    if (!global.BroadcastChannel) {
      global.BroadcastChannel = BroadcastChannel;
    }
    if (!global.TransformStream) {
      global.TransformStream = TransformStream;
    }

    // If a test actually depends on a working ResizeObserver implementation, call
    // mockResizeObserver provided by jsdom-testing-mocks.
    if (!global.ResizeObserver) {
      function NullResizeObserver() {
        this.observe = () => {};
        this.unobserve = () => {};
        this.disconnect = () => {};
      }

      global.ResizeObserver = NullResizeObserver;
    }

    if (!global.navigator.permissions) {
      global.navigator.permissions = {
        query: async () => ({
          onchange: () => {},
        }),
      };
    }

    if (!global.WritableStream) {
      global.WritableStream = WritableStream;
    }

    // JSDOM doesn't provide fetch. We use Node's native fetch instead of the
    // whatwg-fetch polyfill in tests because whatwg-fetch's XHR-based
    // implementation doesn't read response bodies correctly when intercepted
    // by MSW v2. whatwg-fetch's import is a no-op when these globals already
    // exist, so production code that imports it still works.
    //
    // AbortController/AbortSignal must come from the same realm as fetch,
    // otherwise undici rejects the signal as not being an instance of its
    // AbortSignal.
    if (!global.fetch) {
      global.fetch = fetch;
      global.Request = Request;
      global.Response = Response;
      global.Headers = Headers;
      global.AbortController = AbortController;
      global.AbortSignal = AbortSignal;
    }
  }
}
export const TestEnvironment = PatchedJSDOMEnvironment;
