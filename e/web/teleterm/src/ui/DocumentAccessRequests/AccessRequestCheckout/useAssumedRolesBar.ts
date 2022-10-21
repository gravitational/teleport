import { useState, useCallback } from 'react';
import {
  intervalToDuration,
  isBefore,
  secondsToMilliseconds,
  formatDuration,
  // Duration not found in 'date-fns' - false positive for some reason
  // eslint-disable-next-line import/named
  Duration,
} from 'date-fns';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { retryWithRelogin } from 'teleterm/ui/utils';
import { AssumedRequest } from 'teleterm/services/tshd/types';
import { useAsync } from 'shared/hooks/useAsync';
import { useInterval } from 'shared/hooks';

export function useAssumedRolesBar(assumedRequest: AssumedRequest) {
  const ctx = useAppContext();
  const rootClusterUri = ctx.workspacesService?.getRootClusterUri();
  const activeDocUri = ctx.workspacesService
    ?.getActiveWorkspaceDocumentService()
    ?.getActive()?.uri;

  const [duration, setDuration] = useState<Duration>(() =>
    getDurationFromNow({
      end: assumedRequest.expires,
    })
  );
  const [interval, setInterval] = useState<number | null>(
    getRefreshInterval(duration)
  );

  const [dropRequestAttempt, dropRequest] = useAsync(() => {
    // because our bar is outside the `DocumentsRenderer` there
    // is a chance that no document will exist at all. in this case,
    // we let retryWithLogin go down the path as if the originating doc
    // is not active anymore.
    return retryWithRelogin(ctx, activeDocUri, rootClusterUri, () =>
      // only passing the 'unassumed' role id as the backend will
      // persist any other access requests currently available that
      // are not present in the dropIds array
      ctx.clustersService.assumeRole(rootClusterUri, [], [assumedRequest.id])
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
