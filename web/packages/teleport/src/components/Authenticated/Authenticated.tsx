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

import React, { PropsWithChildren, useEffect } from 'react';

import { Box, Indicator } from 'design';
import { TrustedDeviceRequirement } from 'gen-proto-ts/teleport/legacy/types/trusted_device_requirement_pb';
import useAttempt from 'shared/hooks/useAttemptNext';
import Logger from 'shared/libs/logger';
import { getErrMessage } from 'shared/utils/errorType';
import { throttle } from 'shared/utils/highbar';

import { StyledIndicator } from 'teleport/Main';
import { ApiError } from 'teleport/services/api/parseError';
import { storageService } from 'teleport/services/storageService';
import session from 'teleport/services/websession';

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

const Authenticated: React.FC<PropsWithChildren> = ({ children }) => {
  const { attempt, setAttempt } = useAttempt('processing');

  useEffect(() => {
    const checkIfUserIsAuthenticated = async () => {
      if (!session.isValid()) {
        logger.warn('invalid session');
        session.clearBrowserSession(true /* rememberLocation */);
        return;
      }

      try {
        const result = await session.validateCookieAndSession();
        if (result.hasDeviceExtensions) {
          session.setIsDeviceTrusted();
        }
        if (result.requiresDeviceTrust === TrustedDeviceRequirement.REQUIRED) {
          session.setDeviceTrustRequired();
        }
        storageService.setLoginTimeOnce();
        setAttempt({ status: 'success' });
      } catch (e) {
        if (e instanceof ApiError && e.response?.status == 403) {
          logger.warn('invalid session');
          session.clearBrowserSession(true /* rememberLocation */);
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
    session.logoutWithoutSlo();
    return;
  }

  // Initialize or renew the storage before starting interval.
  storageService.setLastActive(Date.now());

  const intervalId = setInterval(() => {
    if (isInactive(adjustedTtl)) {
      logger.warn('inactive session');
      session.logoutWithoutSlo();
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
