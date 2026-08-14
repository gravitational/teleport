import { differenceInMilliseconds, intervalToDuration } from 'date-fns';
import { useEffect, useState } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';

import TeleportContext from 'e-teleport/teleportContextE';
import history from 'teleport/services/history';
import session from 'teleport/services/websession';

export default function useSwitchback(ctx: TeleportContext) {
  const { attempt, setAttempt } = useAttempt();
  const [time, setTime] = useState<Time>({ hours: 0, minutes: 0, seconds: 0 });
  const [btnSetting, setBtnSetting] = useState<BtnSetting>({
    text: 'Switch Back',
    func: onSwitchBack,
  });

  const assumedRoles = ctx.storeAccessRequests.getAssumedRoles();

  useEffect(() => {
    setDuration();

    // Prior state for users who went through the waiting room is the login screen,
    // so it wouldn't make sense for the user to be able to "switch back" to login screen.
    // NOTE: Users do first sign in with static assigned roles before being shown the waiting room,
    // so they can technically "switch back" to this role, but in UI we are treating it as if
    // they have no roles assigned yet.
    if (ctx.storeAccessRequests.getWaitingRoom().state === 'APPLIED') {
      setBtnSetting({
        text: 'Logout',
        func: () => session.logout(),
      });
    }

    // Default update countdown every 15 sec.
    const id = setInterval(setDuration, 15000);

    return () => {
      clearInterval(id);
    };
  }, []);

  function setDuration() {
    const accessRequestExpiry = ctx.storeAccessRequests.getSessionExpiry();

    // Expiry is retrieved from local storage. When user switches back,
    // this expiry will be set to null. Checking for this null value before
    // re-rendering will handle users switching back in one of multiple tabs
    // opened with the switchback rendered.
    if (!accessRequestExpiry) {
      history.reload();
      return;
    }
    const start = new Date();
    const end = new Date(accessRequestExpiry);
    const duration = intervalToDuration({ start, end });

    if (differenceInMilliseconds(end, start) <= 0) {
      session.logout();
    }

    setTime({
      hours: duration.hours,
      minutes: duration.minutes,
      seconds: duration.seconds,
    });
  }

  function onSwitchBack() {
    setAttempt({ status: 'processing' });
    ctx.workflowService
      .applyPermission({ switchback: true })
      .then(() => {
        ctx.storeAccessRequests.clearAssumes();
        history.reload();
      })
      .catch((err: Error) =>
        setAttempt({ status: 'failed', statusText: err.message })
      );
  }

  function onErrorConfirm() {
    setAttempt({ status: '', statusText: '' });
  }

  return {
    assumedRoles,
    time,
    btnSetting,
    attempt,
    onErrorConfirm,
  };
}

type Time = {
  hours: number;
  minutes: number;
  seconds: number;
};

type BtnSetting = {
  text: string;
  func(): void;
};

export type State = ReturnType<typeof useSwitchback>;
