/**
 * Copyright 2020 Gravitational, Inc.
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

/**
 * copyToClipboard copies text to clipboard.
 *
 * @param textToCopy the text to copy to clipboard
 */
export default function copyToClipboard(textToCopy: string): Promise<any> {
  // DELETE when navigator.clipboard is not a working draft
  if (fallbackCopyToClipboard(textToCopy)) {
    return Promise.resolve();
  }

  return navigator.clipboard.writeText(textToCopy).catch(err => {
    // This can happen if the user denies clipboard permissions:
    window.prompt('Cannot copy to clipboard. Use ctrl/cmd + c', err);
  });
}

/**
 * fallbackCopyToClipboard is used when navigator.clipboard is not supported.
 * Note: document.execCommand is marked obselete.
 *
 * @param textToCopy the text to copy to clipboard
 */
function fallbackCopyToClipboard(textToCopy: string): boolean {
  let aux = document.createElement('textarea');
  aux.value = textToCopy;
  document.body.appendChild(aux);
  aux.select();

  // returns false if the command is not supported or enabled
  let isSuccess = document.execCommand('copy');
  document.body.removeChild(aux);

  return isSuccess;
}
