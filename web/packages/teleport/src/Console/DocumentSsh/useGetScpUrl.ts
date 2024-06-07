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

import { useCallback } from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';

import cfg, { UrlScpParams } from 'teleport/config';
import auth, { MfaChallengeScope } from 'teleport/services/auth/auth';

export default function useGetScpUrl(addMfaToScpUrls: boolean) {
  const { setAttempt, attempt, handleError } = useAttempt('');

  const getScpUrl = useCallback(
    async (params: UrlScpParams) => {
      setAttempt({
        status: 'processing',
        statusText: '',
      });
      if (!addMfaToScpUrls) {
        return cfg.getScpUrl(params);
      }
      try {
        let webauthn = await auth.getWebauthnResponse(
          MfaChallengeScope.USER_SESSION
        );
        setAttempt({
          status: 'success',
          statusText: '',
        });
        return cfg.getScpUrl({
          webauthn,
          ...params,
        });
      } catch (error) {
        handleError(error);
      }
    },
    [addMfaToScpUrls, handleError, setAttempt]
  );

  return {
    getScpUrl,
    attempt,
  };
}
