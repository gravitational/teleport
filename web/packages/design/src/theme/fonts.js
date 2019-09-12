/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import { getPlatform } from './utils';

const fontMonoLinux = `"Droid Sans Mono", "monospace", monospace, "Droid Sans Fallback"`;
const fontMonoWin = `Consolas, "Courier New", monospace`;
const fontMonoMac = `Menlo, Monaco, "Courier New", monospace`;

export const font = `Ubuntu2, -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif, "Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol";`

export const fonts = {
  sansSerif: font,
  mono: getMonoFont()
}

function getMonoFont() {
  const platform = getPlatform();

  if (platform.isLinux) {
    return fontMonoLinux;
  }

  if (platform.isMac) {
    return fontMonoMac;
  }

  if (platform.isWin) {
    return fontMonoWin;
  }

  return fontMonoLinux;
}