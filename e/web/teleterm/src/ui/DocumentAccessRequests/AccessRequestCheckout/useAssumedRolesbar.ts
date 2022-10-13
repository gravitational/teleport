import { useState, useEffect } from 'react';

import { intervalToDuration, differenceInMilliseconds } from 'date-fns';

import { useAppContext } from 'teleterm/ui/appContextProvider';

import useAttempt from 'shared/hooks/useAttemptNext';
import { retryWithRelogin } from 'teleterm/ui/utils';
import { AssumedRequest } from 'teleterm/services/tshd/types';

export default function useAssumedRolesBar(assumedRequest: AssumedRequest) {
  const ctx = useAppContext();
  const clusterUri =
    ctx.workspacesService?.getActiveWorkspace()?.localClusterUri;
  const rootClusterUri = ctx.workspacesService?.getRootClusterUri();
  const activeDoc = ctx.workspacesService
    ?.getActiveWorkspaceDocumentService()
    ?.getActive();

  const [time, setTime] = useState<Time>({ hours: 0, minutes: 0, seconds: 0 });
  const { attempt: switchBackAttempt, run: runSwitchBack } = useAttempt('');

  function setDuration() {
    const start = new Date();
    const end = assumedRequest.expires;
    const duration = intervalToDuration({ start, end });

    // tsh certs will always be the shortest lived expiry
    // between 'default' cert or assumed request so if something
    // here expires that means the cert is expired too, regardless
    // of other requests assumed
    if (differenceInMilliseconds(end, start) <= 0) {
      retryWithRelogin(ctx, activeDoc?.uri, clusterUri, () =>
        ctx.clustersService.syncCluster(clusterUri)
      );
    } else {
      setTime({
        hours: duration.hours,
        minutes: duration.minutes,
        seconds: duration.seconds,
      });
    }
  }

  function switchBack() {
    runSwitchBack(() =>
      // because our bar is outside of the documentrenderer there
      // is a chance that no document will exist at all. in this case,
      // we let retryWithLogin go down the path as if the originating doc
      // is not active anymore.
      retryWithRelogin(ctx, activeDoc?.uri, clusterUri, () =>
        // only passing the 'unassumed' role id as the backend will
        // persist any other access requests currently available that
        // are not present in the dropIds array
        ctx.clustersService
          .assumeRole(rootClusterUri, [], [assumedRequest.id])
          .then(() => ctx.clustersService.syncCluster(clusterUri))
          .catch(err => {
            ctx.notificationsService.notifyError({
              title: 'Failed',
              description: err.message,
            });
          })
      )
    );
  }

  useEffect(() => {
    setDuration();

    // Default update countdown every 15 sec.
    const id = setInterval(setDuration, 15000);

    return () => {
      clearInterval(id);
    };
  }, []);

  return {
    time,
    switchBack,
    switchBackAttempt,
    assumedRoles: assumedRequest.roles,
  };
}

type Time = {
  hours: number;
  minutes: number;
  seconds: number;
};
