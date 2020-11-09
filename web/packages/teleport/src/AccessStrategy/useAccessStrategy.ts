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

import React from 'react';
import sessionStorage from 'teleport/services/localStorage';
import useAttempt from 'shared/hooks/useAttempt';
import historyService from 'teleport/services/history';
import userService, {
  AccessStrategy,
  AccessRequest,
  makeAccessRequest,
} from 'teleport/services/user';

export default function useAccessStrategy() {
  const [attempt, attemptActions] = useAttempt({ isProcessing: true });
  const [strategy, setStrategy] = React.useState<AccessStrategy>(null);
  const [accessRequest, setAccessRequest] = React.useState<AccessRequest>(
    makeAccessRequest(sessionStorage.getAccessRequestResult())
  );

  React.useEffect(() => {
    attemptActions.do(() =>
      userService.fetchUserContext().then(res => {
        setStrategy(res.accessStrategy);
        if (
          accessRequest.state === '' &&
          res.accessStrategy.type === 'always'
        ) {
          return createRequest();
        }
      })
    );
  }, []);

  function refresh() {
    return userService
      .fetchAccessRequest(accessRequest.id)
      .then(updateState)
      .catch(attemptActions.error);
  }

  function createRequest(reason?: string) {
    return userService.createAccessRequest(reason).then(updateState);
  }

  function updateState(result: AccessRequest) {
    sessionStorage.setAccessRequestResult(result);
    if (result.state === 'APPROVED') {
      return userService.applyPermission(result.id).then(() => {
        result.state = 'APPLIED';
        sessionStorage.setAccessRequestResult(result);
        historyService.reload();
      });
    }

    setAccessRequest(result);
  }

  return {
    attempt,
    accessRequest,
    strategy,
    refresh,
    createRequest,
  };
}

export type State = ReturnType<typeof useAccessStrategy>;
