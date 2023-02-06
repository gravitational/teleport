import { FileStorage } from 'teleterm/services/fileStorage';

export function createMockFileStorage(): FileStorage {
  let state = {};
  return {
    put(path: string, json: any) {
      state[path] = json;
    },

    get<T>(key: string): T {
      return state[key] as T;
    },

    putAllSync() {},
  };
}
