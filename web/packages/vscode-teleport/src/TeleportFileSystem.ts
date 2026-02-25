import { execFile } from 'child_process';
import { promisify } from 'util';

import {
  Disposable,
  Event,
  EventEmitter,
  FileChangeEvent,
  FileStat,
  FileSystemError,
  FileSystemProvider,
  FileType,
  Uri,
} from 'vscode';

export const scheme = 'tctl';

export class TeleportFileSystem implements FileSystemProvider {
  onDidChangeFile = new EventEmitter<FileChangeEvent[]>().event;

  watch(
    uri: Uri,
    options: {
      readonly recursive: boolean;
      readonly excludes: readonly string[];
    }
  ): Disposable {
    return new Disposable(() => {});
  }

  stat(uri: Uri): FileStat | Thenable<FileStat> {
    return {
      type: FileType.File,
      ctime: 0,
      mtime: 0,
      size: 0,
    };
  }

  async readDirectory(uri: Uri): Promise<[string, FileType][]> {
    if (uri.path === '/') {
      return (await fetchResourceKinds()).map(k => [k, FileType.Directory]);
    }

    const segments = uri.path.split('/').filter(s => !!s);
    if (segments.length !== 1) {
      throw FileSystemError.FileNotFound(uri);
    }

    return (await fetchResourceNames(segments[0])).map(name => [
      name,
      FileType.File,
    ]);
  }

  createDirectory(uri: Uri): void | Thenable<void> {
    throw new Error('Method not implemented.');
  }

  readFile(uri: Uri): Uint8Array | Thenable<Uint8Array> {
    throw new Error('Method not implemented.');
  }

  writeFile(
    uri: Uri,
    content: Uint8Array,
    options: { readonly create: boolean; readonly overwrite: boolean }
  ): void | Thenable<void> {
    throw new Error('Method not implemented.');
  }

  delete(
    uri: Uri,
    options: { readonly recursive: boolean }
  ): void | Thenable<void> {
    throw new Error('Method not implemented.');
  }

  rename(
    oldUri: Uri,
    newUri: Uri,
    options: { readonly overwrite: boolean }
  ): void | Thenable<void> {
    throw new Error('Method not implemented.');
  }
}

async function tctl(args: string[]): Promise<string> {
  return (
    await promisify(execFile)('/Users/bartosz/code/teleport/build/tctl', args)
  ).stdout;
}

async function fetchResourceKinds(): Promise<string[]> {
  const kinds = (await tctl(['list-kinds', '--wide']))
    .split('\n')
    .slice(2)
    .map(line => line.match(/^(\S+)/)?.[1] ?? '')
    .filter(k => !!k);
  kinds.sort((a, b) => a.localeCompare(b));
  return kinds;
}

type Resource = {
  metadata: {
    name: string;
  };
};

async function fetchResourceNames(kind: string): Promise<string[]> {
  const text = await tctl(['get', kind, '--format=json']);
  try {
    const json = JSON.parse(text);
  } catch (e) {
    console.log(text);
    throw e;
  }
  return JSON.parse(text).map((res: Resource) => res.metadata.name);
}
