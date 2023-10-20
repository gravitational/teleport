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
  }
}
export const TestEnvironment = PatchedJSDOMEnvironment;
