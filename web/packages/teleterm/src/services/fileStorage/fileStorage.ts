import fs, { readFileSync, existsSync, writeFileSync } from 'fs';
import Logger from 'teleterm/logger';

const logger = new Logger('FileStorage');

export interface FileStorage {
  put(path: string, json: any): void;
  get<T>(path: string): T;
}

export function createFileStorage(opts: { filePath: string }): FileStorage {
  if (!opts || !opts.filePath) {
    throw Error('missing filePath');
  }

  const { filePath } = opts;
  const state = loadState(opts.filePath);

  function put(key: string, json: any) {
    state[key] = json;
    const text = JSON.stringify(state, null, 2);
    fs.promises.writeFile(filePath, text).catch(error => {
      logger.error(`Cannot update ${filePath} file`, error);
    });
  }

  function get<T>(key: string): T {
    return state[key] as T;
  }

  return {
    put,
    get,
  };
}

function loadState(filePath: string) {
  try {
    if (!existsSync(filePath)) {
      writeFileSync(filePath, '{}');
    }

    return JSON.parse(readFileSync(filePath, { encoding: 'utf-8' }));
  } catch (error) {
    logger.error(`Cannot read ${filePath} file`, error);
    return {};
  }
}
