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

import {
  createFileTransferEventsEmitter,
  FileTransferEventsEmitter,
  FileTransferListeners,
} from 'shared/components/FileTransfer';

import { getAuthHeaders, getNoCacheHeaders } from 'teleport/services/api';

export function getHttpFileTransferHandlers() {
  return {
    upload(
      url: string,
      file: File,
      abortController?: AbortController
    ): FileTransferListeners {
      const eventEmitter = createFileTransferEventsEmitter();
      const xhr = getBaseXhrRequest({
        method: 'post',
        url,
        eventEmitter,
        abortController,
        transformFailedResponse: () => getErrorText(xhr.response),
      });

      xhr.upload.addEventListener('progress', e => {
        eventEmitter.emitProgress(calculateProgress(e));
      });
      xhr.send(file);
      return eventEmitter;
    },
    download(
      url: string,
      abortController?: AbortController
    ): FileTransferListeners {
      const eventEmitter = createFileTransferEventsEmitter();
      const xhr = getBaseXhrRequest({
        method: 'get',
        url,
        eventEmitter,
        abortController,
        transformSuccessfulResponse: () => {
          const fileName = getDispositionFileName(xhr);
          if (!fileName) {
            throw new Error('Bad response');
          } else {
            saveOnDisk(fileName, xhr.response);
          }
        },
        transformFailedResponse: () => getFileReaderErrorAsText(xhr.response),
      });

      xhr.onprogress = e => {
        if (xhr.status === 200) {
          eventEmitter.emitProgress(calculateProgress(e));
        }
      };
      xhr.responseType = 'blob';
      xhr.send();
      return eventEmitter;
    },
  };
}

function getBaseXhrRequest({
  method,
  url,
  abortController,
  eventEmitter,
  transformSuccessfulResponse,
  transformFailedResponse,
}: {
  method: string;
  url: string;
  eventEmitter: FileTransferEventsEmitter;
  abortController: AbortController;
  transformSuccessfulResponse?(): void;
  transformFailedResponse?(): Promise<string> | string;
}): XMLHttpRequest {
  function setHeaders(): void {
    const headers = {
      ...getAuthHeaders(),
      ...getNoCacheHeaders(),
    };

    Object.keys(headers).forEach(key => {
      xhr.setRequestHeader(key, headers[key]);
    });
  }

  function attachHandlers(): void {
    if (abortController) {
      abortController.signal.onabort = () => {
        xhr.abort();
      };
    }

    xhr.onload = async () => {
      if (xhr.status !== 200) {
        eventEmitter.emitError(new Error(await transformFailedResponse()));
        return;
      }

      try {
        transformSuccessfulResponse?.();
        eventEmitter.emitComplete();
      } catch (error) {
        eventEmitter.emitError(error);
      }
    };

    xhr.onerror = async () => {
      eventEmitter.emitError(new Error(await transformFailedResponse()));
    };

    xhr.ontimeout = () => {
      eventEmitter.emitError(new Error('Request timed out.'));
    };

    xhr.onabort = () => {
      eventEmitter.emitError(new DOMException('Aborted', 'AbortError'));
    };
  }

  const xhr = new XMLHttpRequest();
  xhr.open(method, url, true);
  setHeaders();
  attachHandlers();

  return xhr;
}

function getFileReaderErrorAsText(xhrResponse: Blob): Promise<string> {
  return new Promise(resolve => {
    const reader = new FileReader();

    reader.onerror = () => {
      resolve(reader.error.message);
    };

    reader.onload = () => {
      const text = getErrorText(reader.result as string);
      resolve(text);
    };

    reader.readAsText(xhrResponse);
  });
}

function saveOnDisk(fileName: string, blob: Blob): void {
  const a = document.createElement('a');
  a.href = window.URL.createObjectURL(blob);
  a.download = fileName;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
}

// backend may return errors in different formats,
// look at different JSON structures to retrieve the error message
function getErrorText(response: string | undefined): string {
  const badRequest = 'Bad request';
  if (!response) {
    return badRequest;
  }

  try {
    const json = JSON.parse(response);
    return json.error?.message || json.message || badRequest;
  } catch {
    return 'Bad request, failed to parse error message.';
  }
}

function calculateProgress(e: ProgressEvent): number {
  // if Content-Length is present
  if (e.lengthComputable) {
    return Math.round((e.loaded / e.total) * 100);
  } else {
    const done = e.loaded;
    const total = e.total;
    return Math.floor((done / total) * 1000) / 10;
  }
}

function getDispositionFileName(xhr: XMLHttpRequest) {
  let fileName = '';
  const disposition = xhr.getResponseHeader('Content-Disposition');
  if (disposition) {
    const filenameRegex = /filename[^;=\n]*=((['"]).*?\2|[^;\n]*)/;
    const matches = filenameRegex.exec(disposition);
    if (matches != null && matches[1]) {
      fileName = matches[1].replace(/['"]/g, '');
    }
  }

  return decodeURIComponent(fileName);
}
