import { TextEncoder, TextDecoder } from 'node:util';

import { TestEnvironment as JSDOMEnvironment } from 'jest-environment-jsdom';

// When using jest-environment-jsdom, TextEncoder and TextDecoder are not defined. This poses a
// problem when writing tests for code which uses TextEncoder and TextDecoder directly or that
// imports libraries which depend on those globals (for example whatwg-url).
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
      global.structuredClone = val => {
        return JSON.parse(JSON.stringify(val));
      };
    }

    // TODO(gzdunek): Remove this once JSDOM provides scrollIntoView.
    // https://github.com/jsdom/jsdom/issues/1695#issuecomment-449931788
    if (!global.Element.prototype.scrollIntoView) {
      global.Element.prototype.scrollIntoView = () => {};
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
  }
}
export const TestEnvironment = PatchedJSDOMEnvironment;
