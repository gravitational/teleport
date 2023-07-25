/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

export const compareSemVers = (a: string, b: string): -1 | 1 => {
  const splitA = a.split('.');
  const splitB = b.split('.');

  if (splitA.length < 3 || splitB.length < 3) {
    return -1;
  }

  const majorA = parseInt(splitA[0]);
  const majorB = parseInt(splitB[0]);
  if (majorA !== majorB) {
    return majorA > majorB ? 1 : -1;
  }

  const minorA = parseInt(splitA[1]);
  const minorB = parseInt(splitB[1]);
  if (minorA !== minorB) {
    return minorA > minorB ? 1 : -1;
  }

  const patchA = parseInt(splitA[2].split('-')[0]);
  const patchB = parseInt(splitB[2].split('-')[0]);
  if (patchA !== patchB) {
    return patchA > patchB ? 1 : -1;
  }

  return 1;
};
