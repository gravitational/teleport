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

const DOWNLOAD_BASE_URL = 'https://get.gravitational.com/';

export default function getDownloadLink(
  type: Arch,
  version: string,
  isEnterprise: boolean
) {
  let infix = 'linux-amd64';
  const enterprise = isEnterprise ? 'ent-' : '';
  if (type === 'mac') {
    return `${DOWNLOAD_BASE_URL}teleport-${enterprise}${version}.pkg`;
  } else if (type === 'linux32') {
    infix = 'linux-386';
  }

  return `${DOWNLOAD_BASE_URL}teleport-${enterprise}v${version}-${infix}-bin.tar.gz`;
}

type Arch = 'mac' | 'linux32' | 'linux64';
