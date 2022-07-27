// Copyright 2022 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// SharedDirectoryManager manages a FileSystemDirectoryHandle for use
// by the TDP client. Most of its methods can potentially throw errors
// and so should be wrapped in try/catch blocks.
export class SharedDirectoryManager {
  private dir: FileSystemDirectoryHandle | undefined;

  add(sharedDirectory: FileSystemDirectoryHandle) {
    if (this.dir) {
      throw new Error(
        'SharedDirectoryManager currently only supports sharing a single directory'
      );
    }
    this.dir = sharedDirectory;
  }

  getName(): string {
    this.checkReady();
    return this.dir.name;
  }

  // Gets the information for the file or directory
  // at path where path is the relative path from the
  // root directory.
  async getInfo(path: string): Promise<FileOrDirInfo> {
    this.checkReady();

    const fileOrDir = await this.walkPath(path);

    if (fileOrDir.kind === 'directory') {
      // Magic numbers are the values for directories where the true
      // value is unavailable, according to the TDP spec.
      return { size: 4096, lastModified: 0, kind: fileOrDir.kind, path };
    }

    let file = await fileOrDir.getFile();
    return {
      size: file.size,
      lastModified: file.lastModified,
      kind: fileOrDir.kind,
      path,
    };
  }

  // Gets the FileOrDirInfo for all the children of the
  // directory at path.
  async listContents(path: string): Promise<FileOrDirInfo[]> {
    this.checkReady();

    // Get the directory whose contents we want to list.
    const dir = await this.walkPath(path);
    if (dir.kind !== 'directory') {
      throw new Error('cannot list the contents of a file');
    }

    let infos: FileOrDirInfo[] = [];
    for await (const entry of dir.values()) {
      // Create the full relative path to the entry
      let entryPath = path;
      if (entryPath !== '') {
        entryPath = [entryPath, entry.name].join('/');
      } else {
        entryPath = entry.name;
      }
      infos.push(await this.getInfo(entryPath));
    }

    return infos;
  }

  // walkPath walks a pathstr (assumed to be in the qualified Unix format specified
  // in the TDP spec), returning the FileSystemDirectoryHandle | FileSystemFileHandle
  // it finds at its end. If the pathstr isn't a valid path in the shared directory,
  // it throws an error.
  private async walkPath(
    pathstr: string
  ): Promise<FileSystemDirectoryHandle | FileSystemFileHandle> {
    if (pathstr === '') {
      return this.dir;
    }

    let path = pathstr.split('/');

    let walkIt = async (
      dir: FileSystemDirectoryHandle,
      path: string[]
    ): Promise<FileSystemDirectoryHandle | FileSystemFileHandle> => {
      // Pop the next path element off the stack
      let nextPathElem = path.shift();

      // Iterate through the items in the directory
      for await (const entry of dir.values()) {
        // If we find the entry we're looking for
        if (entry.name === nextPathElem) {
          if (path.length === 0) {
            // We're at the end of the path, so this
            // is the end element we've been walking towards.
            return entry;
          } else if (entry.kind === 'directory') {
            // We're not at the end of the path and
            // have encountered a directory, recurse
            // further.
            return walkIt(entry, path);
          } else {
            break;
          }
        }
      }

      throw new PathDoesNotExistError('path does not exist');
    };

    return walkIt(this.dir, path);
  }

  private checkReady() {
    if (!this.dir) {
      throw new Error(
        'attempted to use a shared directory before one was initialized'
      );
    }
  }
}

export class PathDoesNotExistError extends Error {
  constructor(message: string) {
    super(message);
  }
}

export type FileOrDirInfo = {
  size: number; // bytes
  lastModified: number; // ms since unix epoch
  kind: 'file' | 'directory';
  path: string;
};
