/*
Copyright 2022 Gravitational, Inc.

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

import { useEffect, useState } from 'react';
import { formatDistanceStrict } from 'date-fns';
import useAttempt from 'shared/hooks/useAttemptNext';
import TeleportContext from 'teleport/teleportContext';

export default function useAddDatabase(ctx: TeleportContext) {
  const { attempt, run } = useAttempt('processing');
  const [expiry, setExpiry] = useState('');
  const [token, setToken] = useState('');

  useEffect(() => {
    createJoinToken();
  }, []);

  function createJoinToken() {
    return run(() =>
      ctx.joinTokenService.fetchJoinToken(['Db']).then(token => {
        const expires = formatDistanceStrict(
          new Date(),
          new Date(token.expiry)
        );
        setExpiry(expires);
        setToken(token.id);
      })
    );
  }

  return {
    createJoinToken,
    expiry,
    attempt,
    token,
  };
}

export type State = ReturnType<typeof useAddDatabase>;
