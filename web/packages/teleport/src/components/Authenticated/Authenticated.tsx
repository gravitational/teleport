/*
Copyright 2019-2021 Gravitational, Inc.

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

import React, { useEffect } from 'react';
import { throttle } from 'shared/utils/highbar';
import Logger from 'shared/libs/logger';
import useAttempt from 'shared/hooks/useAttemptNext';
import { getErrMessage } from 'shared/utils/errorType';
import { Box, Indicator } from 'design';

import session from 'teleport/services/websession';
import { storageService } from 'teleport/services/storageService';
import { ApiError } from 'teleport/services/api/parseError';
import { StyledIndicator } from 'teleport/Main';

import { ErrorDialog } from './ErrorDialogue';

const logger = Logger.create('/components/Authenticated');
const ACTIVITY_CHECKER_INTERVAL_MS = 30 * 1000;
const ACTIVITY_EVENT_DELAY_MS = 15 * 1000;

const events = [
  // Fired from any keyboard key press.
  'keydown',
  // Fired when a pointer (cursor, pen/stylus, touch) changes coordinates.
  // This also handles mouse scrolling. It's unlikely a user will keep their
  // mouse still when scrolling.
  'pointermove',
  // Fired when a pointer (cursor, pen/stylus, touch) becomes active button
  // states (ie: mouse clicks or pen/finger has physical contact with touch enabled screen).
  'pointerdown',
];

const Authenticated: React.FC = ({ children }) => {
  const { attempt, setAttempt } = useAttempt('processing');

  useEffect(() => {
    const checkIfUserIsAuthenticated = async () => {
      if (!session.isValid()) {
        logger.warn('invalid session');
        session.logout(true /* rememberLocation */);
        return;
      }

      try {
        await session.validateCookieAndSession();
        setAttempt({ status: 'success' });
      } catch (e) {
        if (e instanceof ApiError && e.response?.status == 403) {
          logger.warn('invalid session');
          session.logout(true /* rememberLocation */);
          // No need to update attempt, as `logout` will
          // redirect user to login page.
          return;
        }
        // Error unrelated to authentication failure (network blip).
        setAttempt({ status: 'failed', statusText: getErrMessage(e) });
      }
    };

    checkIfUserIsAuthenticated();
  }, []);

  useEffect(() => {
    if (attempt.status !== 'success') {
      return;
    }

    session.ensureSession();

    const inactivityTtl = session.getInactivityTimeout();
    if (inactivityTtl === 0) {
      return;
    }

    return startActivityChecker(inactivityTtl);
  }, [attempt.status]);

  if (attempt.status === 'success') {
    return <>{children}</>;
  }

  if (attempt.status === 'failed') {
    return <ErrorDialog errMsg={attempt.statusText} />;
  }

  return (
    <Box textAlign="center">
      <StyledIndicator>
        <Indicator />
      </StyledIndicator>
    </Box>
  );
};

export default Authenticated;

function startActivityChecker(ttl = 0) {
  // adjustedTtl slightly improves accuracy of inactivity time.
  // This will at most cause user to log out ACTIVITY_CHECKER_INTERVAL_MS early.
  // NOTE: Because of browser js throttling on inactive tabs, expiry timeout may
  // still be extended up to over a minute.
  const adjustedTtl = ttl - ACTIVITY_CHECKER_INTERVAL_MS;

  // See if there is inactive date already set in local storage.
  // This is to check for idle timeout reached while app was closed
  // ie. browser still openend but all app tabs closed.
  if (isInactive(adjustedTtl)) {
    logger.warn('inactive session');
    session.logout();
    return;
  }

  // Initialize or renew the storage before starting interval.
  storageService.setLastActive(Date.now());

  const intervalId = setInterval(() => {
    if (isInactive(adjustedTtl)) {
      logger.warn('inactive session');
      session.logout();
    }
  }, ACTIVITY_CHECKER_INTERVAL_MS);

  const throttled = throttle(() => {
    storageService.setLastActive(Date.now());
  }, ACTIVITY_EVENT_DELAY_MS);

  events.forEach(event => window.addEventListener(event, throttled));

  function stop() {
    throttled.cancel();
    clearInterval(intervalId);
    events.forEach(event => window.removeEventListener(event, throttled));
  }

  return stop;
}

function isInactive(ttl = 0) {
  const lastActive = storageService.getLastActive();
  return lastActive > 0 && Date.now() - lastActive > ttl;
}
