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

import moment from 'moment';
import cfg from 'teleport/config';
import { NodeToken, BashCommand } from './types';

export default function makeAppBashCmd(
  token: NodeToken,
  appName = '',
  appUri = ''
): BashCommand {
  const duration = moment(new Date()).diff(token.expiry);
  const expires = moment.duration(duration).humanize();

  // encode uri so it can be passed around as URL query parameter
  const encoded = encodeURIComponent(appUri)
    // encode single quotes so they do not break the curl parameters
    .replace(/'/g, '%27');

  const bashUrl =
    cfg.baseUrl +
    cfg.api.appNodeScriptPath
      .replace(':token', token.id)
      .replace(':name', appName)
      .replace(':uri', encoded);

  const text = `sudo bash -c "$(curl -fsSL '${bashUrl}')"`;

  return {
    text,
    expires,
  };
}
