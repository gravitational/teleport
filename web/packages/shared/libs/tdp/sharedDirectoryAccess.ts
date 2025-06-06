/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { FileType } from './codec';

export interface SharedDirectoryAccess {
  /** Prompts the user to select a directory to share. */
  selectDirectory(): Promise<void>;
  /** Returns the name of the currently shared directory. */
  getDirectoryName(): string;
  /** Retrieves metadata about a file or directory at the given path. */
  stat(path: string): Promise<FileOrDirInfo>;
  /** Lists files and directories within the given directory path. */
  readDir(path: string): Promise<FileOrDirInfo[]>;
  /** Reads a slice of a file. */
  read(path: string, offset: bigint, length: number): Promise<Uint8Array>;
  /** Writes data to a file at a given offset. */
  write(path: string, offset: bigint, data: Uint8Array): Promise<number>;
  /** Truncates a file to the specified size. */
  truncate(path: string, size: number): Promise<void>;
  /** Creates a new file or directory at the given path. */
  create(path: string, fileType: FileType): Promise<void>;
  /** Deletes a file or directory at the given path. */
  delete(path: string): Promise<void>;
}

/**
 * Enables directory sharing using FileSystem API.
 * Most of the methods can potentially throw errors and so should be wrapped in try/catch blocks.
 * Should be kept in sync with lib/teleterm/services/desktop/directorysharing.go
 * where file system events are handled for Connect.
 */
export class BrowserFileSystem implements SharedDirectoryAccess {
  private dir: FileSystemDirectoryHandle | undefined;

  /**
   * Opens a directory.
   * @throws Will throw an error if a directory is already being shared.
   */
  async selectDirectory() {
    if (typeof window.showDirectoryPicker !== 'function') {
      // This is a gross error message, but should be infrequent enough that its worth just telling
      // the user the likely problem, while also displaying the error message just in case that's not it.
      // In a perfect world, we could check for which error message this is and display
      // context appropriate directions.
      throw new Error(
        'Your user role supports directory sharing over desktop access, \
  however this feature is only available by default on some Chromium \
  based browsers like Google Chrome or Microsoft Edge. Brave users can \
  use the feature by navigating to brave://flags/#file-system-access-api \
  and selecting "Enable". If you\'re not already, please switch to a supported browser.'
      );
    }

    const sharedDirectory = await window.showDirectoryPicker();
    if (this.dir) {
      throw new Error(
        'SharedDirectoryManager currently only supports sharing a single directory'
      );
    }
    this.dir = sharedDirectory;
  }

  /**
   * @throws Will throw an error if a directory has not already been initialized.
   */
  getDirectoryName(): string {
    this.checkReady();
    return this.dir.name;
  }

  /**
   * Gets the information for the file or directory at path where path is the relative path from the root directory.
   * @throws Will throw an error if a directory has not already been initialized.
   * @throws {PathDoesNotExistError} if the pathstr isn't a valid path in the shared directory
   */
  async stat(path: string): Promise<FileOrDirInfo> {
    this.checkReady();

    const fileOrDir = await this.walkPath(path);

    let isEmpty = true;
    if (fileOrDir.kind === 'directory') {
      let dir = fileOrDir;
      // If dir contains any files or directories, it will
      // enter the loop below and we can register it as not
      // empty. If it doesn't, it will skip over the loop.
      // eslint-disable-next-line unused-imports/no-unused-vars
      for await (const _ of dir.keys()) {
        isEmpty = false;
        break;
      }

      // Magic numbers are the values for directories where the true
      // value is unavailable, according to the TDP spec.
      return {
        size: 4096,
        lastModified: 0,
        kind: fileOrDir.kind,
        isEmpty,
        path,
      };
    }

    let file = await fileOrDir.getFile();
    return {
      size: file.size,
      lastModified: file.lastModified,
      kind: fileOrDir.kind,
      isEmpty,
      path,
    };
  }

  /**
   * Gets the FileOrDirInfo for all the children of the directory at path.
   * @throws Will throw an error if a directory has not already been initialized.
   * @throws {PathDoesNotExistError} if the pathstr isn't a valid path in the shared directory
   */
  async readDir(path: string): Promise<FileOrDirInfo[]> {
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
      infos.push(await this.stat(entryPath));
    }

    return infos;
  }

  /**
   * Reads length bytes starting at offset from a file at path.
   * @throws Will throw an error if a directory has not already been initialized.
   * @throws {PathDoesNotExistError} if the pathstr isn't a valid path in the shared directory
   */
  async read(
    path: string,
    offset: bigint,
    length: number
  ): Promise<Uint8Array> {
    this.checkReady();
    const fileHandle = await this.getFileHandle(path);
    const file = await fileHandle.getFile();
    return new Uint8Array(
      await file.slice(Number(offset), Number(offset) + length).arrayBuffer()
    );
  }

  /**
   * Writes the bytes in writeData to the file at path starting at offset.
   * @throws Will throw an error if a directory has not already been initialized.
   * @throws {PathDoesNotExistError} if the pathstr isn't a valid path in the shared directory
   */
  async write(path: string, offset: bigint, data: Uint8Array): Promise<number> {
    this.checkReady();

    const fileHandle = await this.getFileHandle(path);
    const file = await fileHandle.createWritable({ keepExistingData: true });
    await file.write({ type: 'write', position: Number(offset), data });
    await file.close(); // Needed to actually write data to disk.

    return data.length;
  }

  /**
   * Truncates the file at path to size bytes.
   * @throws Will throw an error if a directory has not already been initialized.
   * @throws {PathDoesNotExistError} if the pathstr isn't a valid path in the shared directory
   */
  async truncate(path: string, size: number): Promise<void> {
    this.checkReady();
    const fileHandle = await this.getFileHandle(path);
    const file = await fileHandle.createWritable({ keepExistingData: true });
    await file.truncate(size);
    await file.close();
  }

  /**
   * Creates a new file or directory (determined by fileType) at path.
   * If the path already exists for the given fileType, this operation is effectively ignored.
   * @throws {DomException} If the path already exists but not for the given fileType.
   * @throws Anything potentially thrown by getFileHandle/getDirectoryHandle.
   * @throws {PathDoesNotExistError} if the path isn't a valid path to a directory.
   */
  async create(path: string, fileType: FileType): Promise<void> {
    let splitPath = path.split('/');
    const fileOrDirName = splitPath.pop();
    const dirPath = splitPath.join('/');
    const dirHandle = await this.getDirectoryHandle(dirPath);
    if (fileType === FileType.File) {
      await dirHandle.getFileHandle(fileOrDirName, { create: true });
    } else {
      await dirHandle.getDirectoryHandle(fileOrDirName, { create: true });
    }
  }

  /**
   * Deletes a file or directory at path.
   * If the path doesn't exist, this operation is effectively ignored.
   * @throws Anything potentially thrown by getFileHandle/getDirectoryHandle.
   * @throws {PathDoesNotExistError} if the path isn't a valid path to a directory.
   */
  async delete(path: string): Promise<void> {
    let splitPath = path.split('/');
    const fileOrDirName = splitPath.pop();
    const dirPath = splitPath.join('/');
    const dirHandle = await this.getDirectoryHandle(dirPath);
    await dirHandle.removeEntry(fileOrDirName, { recursive: true });
  }

  /**
   * Returns the FileSystemFileHandle for the file at path.
   * @throws {PathDoesNotExistError} if the pathstr isn't a valid path in the shared directory
   * @throws {Error} if the pathstr points to a directory
   */
  private async getFileHandle(pathstr: string): Promise<FileSystemFileHandle> {
    const fileHandle = await this.walkPath(pathstr);
    if (fileHandle.kind !== 'file') {
      throw new Error('cannot read the bytes of a directory');
    }
    return fileHandle;
  }

  /**
   * Returns the FileSystemDirectoryHandle for the directory at path.
   * @throws {PathDoesNotExistError} if the pathstr isn't a valid path in the shared directory
   * @throws {Error} if the pathstr points to a file
   */
  private async getDirectoryHandle(
    pathstr: string
  ): Promise<FileSystemDirectoryHandle> {
    const dirHandle = await this.walkPath(pathstr);
    if (dirHandle.kind !== 'directory') {
      throw new Error('cannot list the contents of a file');
    }
    return dirHandle;
  }

  /**
   * walkPath walks a pathstr (assumed to be in the qualified Unix format specified
   * in the TDP spec), returning the FileSystemDirectoryHandle | FileSystemFileHandle
   * it finds at its end.
   * @throws {PathDoesNotExistError} if the pathstr isn't a valid path in the shared directory
   */
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

  /**
   * @throws Will throw an error if a directory has not already been initialized.
   */
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
  isEmpty: boolean;
  path: string;
};
