import { EventEmitter } from 'events';

import { TshAbortController } from './types';

/**
 * Creates a version of AbortController that can be passed through Electron contextBridge
 */
export default function createAbortController(): TshAbortController {
  const emitter = new EventEmitter();

  const signal = {
    addEventListener(cb: (...args: any[]) => void) {
      emitter.addListener('abort', cb);
    },

    removeEventListener(cb: (...args: any[]) => void) {
      emitter.removeListener('abort', cb);
    },
  };

  return {
    signal,
    abort() {
      emitter.emit('abort');
    },
  };
}
