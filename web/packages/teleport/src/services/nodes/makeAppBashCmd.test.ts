/*
Copyright 2020 Gravitational, Inc.

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

import makeAppBashCmd from './makeAppBashCmd';

test('encoding', () => {
  const token = { id: '86', expiry: new Date('2019-05-13T20:18:09Z') };
  const appName = 'jenkins';
  const appUri = `http://myapp/test?b='d'&a="1"&c=|`;

  const cmd = makeAppBashCmd(token, appName, appUri);
  expect(cmd.text).toBe(
    `sudo bash -c "$(curl -fsSL 'http://localhost/scripts/86/install-app.sh?name=jenkins&uri=http%3A%2F%2Fmyapp%2Ftest%3Fb%3D%27d%27%26a%3D%221%22%26c%3D%7C')"`
  );
});
