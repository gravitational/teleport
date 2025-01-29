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

import {
  Duration,
  formatDuration,
  intervalToDuration,
  isBefore,
  secondsToMilliseconds,
} from 'date-fns';
import { useCallback, useState } from 'react';

import { useInterval } from 'shared/hooks';
import { useAsync } from 'shared/hooks/useAsync';

import { AssumedRequest } from 'teleterm/services/tshd/types';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useResourcesContext } from 'teleterm/ui/DocumentCluster/resourcesContext';
import { retryWithRelogin } from 'teleterm/ui/utils';

export function useAssumedRolesBar(assumedRequest: AssumedRequest) {
  const ctx = useAppContext();
  const rootClusterUri = ctx.workspacesService?.getRootClusterUri();
  const { requestResourcesRefresh } = useResourcesContext(rootClusterUri);

  const [duration, setDuration] = useState<Duration>(() =>
    getDurationFromNow({
      end: assumedRequest.expires,
    })
  );
  const [interval, setInterval] = useState<number | null>(
    getRefreshInterval(duration)
  );

  const [dropRequestAttempt, dropRequest] = useAsync(async () => {
    try {
      await retryWithRelogin(ctx, rootClusterUri, () =>
        ctx.clustersService.dropRoles(rootClusterUri, [assumedRequest.id])
      );
      requestResourcesRefresh();
    } catch (err) {
      ctx.notificationsService.notifyError({
        title: 'Could not drop role',
        description: err.message,
      });
    }
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
