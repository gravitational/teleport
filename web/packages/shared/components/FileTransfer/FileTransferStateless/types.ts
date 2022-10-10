/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
