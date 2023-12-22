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

export type TransferState =
  | { type: 'processing'; progress: number }
  | { type: 'error'; error: Error; progress: number }
  | { type: 'completed' };

export type TransferredFile = {
  id: string;
  name: string;
  transferState: TransferState;
};

export type FileTransferListeners = {
  onProgress(callback: (percentage: number) => void): void;
  onError(callback: (error: Error) => void): void;
  onComplete(callback: () => void): void;
};

export enum FileTransferDialogDirection {
  Download = 'Download',
  Upload = 'Upload',
}
