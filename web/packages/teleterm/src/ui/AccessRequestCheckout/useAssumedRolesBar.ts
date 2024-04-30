/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { useState, useCallback } from 'react';
import {
  intervalToDuration,
  isBefore,
  secondsToMilliseconds,
  formatDuration,
  Duration,
} from 'date-fns';

import { useAsync } from 'shared/hooks/useAsync';

import { useInterval } from 'shared/hooks';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { retryWithRelogin } from 'teleterm/ui/utils';
import { AssumedRequest } from 'teleterm/services/tshd/types';

export function useAssumedRolesBar(assumedRequest: AssumedRequest) {
  const ctx = useAppContext();
  const rootClusterUri = ctx.workspacesService?.getRootClusterUri();

  const [duration, setDuration] = useState<Duration>(() =>
    getDurationFromNow({
      end: assumedRequest.expires,
    })
  );
  const [interval, setInterval] = useState<number | null>(
    getRefreshInterval(duration)
  );

  const [dropRequestAttempt, dropRequest] = useAsync(() => {
    return retryWithRelogin(
      ctx,
      rootClusterUri,
      () =>
        // only passing the 'unassumed' role id as the backend will
        // persist any other access requests currently available that
        // are not present in the dropIds array
        ctx.clustersService.assumeRole(rootClusterUri, [], [assumedRequest.id])
      // TODO(gzdunek): We should refresh the resources,
      // the same as after assuming a role in `useAssumeAccess`.
      // Unfortunately, we can't do this because we don't have access to `ResourcesContext`.
      // Consider moving it into `ResourcesService`.
    ).catch(err => {
      ctx.notificationsService.notifyError({
        title: 'Could not switch back the role',
        description: err.message,
      });
    });
  });

  const updateDurationAndInterval = useCallback(() => {
    const calculatedDuration = getDurationFromNow({
      end: assumedRequest.expires,
    });
    setDuration(calculatedDuration);

    if (hasExpired(calculatedDuration)) {
      setInterval(null); // stop updates
    } else {
      setInterval(getRefreshInterval(calculatedDuration));
    }
  }, [assumedRequest.expires]);

  useInterval(updateDurationAndInterval, interval);

  return {
    duration: getFormattedDuration(duration),
    hasExpired: hasExpired(duration),
    dropRequest,
    dropRequestAttempt,
    assumedRoles: assumedRequest.roles,
  };
}

//TODO(gzdunek): use it in web too
function getFormattedDuration(duration: Duration): string {
  const format = ['years', 'months', 'weeks', 'days', 'hours', 'minutes'];

  if (isLessThan10Minutes(duration)) {
    format.push('seconds');
  }

  return formatDuration(duration, {
    format,
  });
}

function hasExpired(duration: Duration): boolean {
  return (
    !duration.years &&
    !duration.months &&
    !duration.weeks &&
    !duration.days &&
    !duration.hours &&
    !duration.minutes &&
    !duration.seconds
  );
}

const HIGH_REFRESH_RATE = secondsToMilliseconds(1);
const LOW_REFRESH_RATE = secondsToMilliseconds(15);

function getRefreshInterval(duration: Duration): number {
  return isLessThan10Minutes(duration) ? HIGH_REFRESH_RATE : LOW_REFRESH_RATE;
}

function getDurationFromNow(params: { end: Date }): Duration {
  const now = new Date();

  if (isBefore(params.end, now)) {
    return {}; // all values are empty
  }

  return intervalToDuration({
    start: now,
    end: params.end,
  });
}

function isLessThan10Minutes(duration: Duration) {
  return (
    !duration.years &&
    !duration.months &&
    !duration.weeks &&
    !duration.days &&
    !duration.hours &&
    duration.minutes < 10
  );
}
