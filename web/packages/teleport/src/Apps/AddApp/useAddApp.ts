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

import { useState } from 'react';
import TeleportContext from 'teleport/teleportContext';
import useAttempt from 'shared/hooks/useAttemptNext';

export default function useAddApp(ctx: TeleportContext) {
  const { attempt, run } = useAttempt('');
  const user = ctx.storeUser.state.username;
  const version = ctx.storeUser.state.cluster.authVersion;
  const isAuthTypeLocal = !ctx.storeUser.isSso();
  const isEnterprise = ctx.isEnterprise;
  const [automatic, setAutomatic] = useState(isEnterprise);
  const [cmd, setCmd] = useState('');
  const [expires, setExpires] = useState('');

  function createToken(appName = '', appUri = '') {
    return run(() =>
      ctx.nodeService.createAppBashCommand(appName, appUri).then(result => {
        setCmd(result.text);
        setExpires(result.expires);
      })
    );
  }

  return {
    user,
    version,
    createToken,
    cmd,
    expires,
    attempt,
    automatic,
    setAutomatic,
    isAuthTypeLocal,
    isEnterprise,
  };
}

export type State = ReturnType<typeof useAddApp>;
