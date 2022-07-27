import { useState, useEffect } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';

import TeleportContextE from 'e-teleport/teleportContextE';
import cfg from 'e-teleport/config';

import type { UpgradeWindowStart } from 'e-teleport/services/upgradeWindow';

export function useUpgradeWindowStart(ctx: TeleportContextE) {
  const { attempt, run } = useAttempt();

  const [scheduleUpgradesVisible, setScheduleUpgradesVisible] = useState(false);
  const [selectedUpgradeWindowStart, setSelectedUpgradeWindowStart] =
    useState<UpgradeWindowStart>('08:00:00');

  useEffect(() => {
    if (!cfg.oss.isCloud) {
      return;
    }

    run(() =>
      ctx.upgradeWindowService
        .getUpgradeWindowStart()
        .then(setSelectedUpgradeWindowStart)
    );
  }, []);

  function showScheduleUpgrade() {
    setScheduleUpgradesVisible(true);
  }

  function closeScheduleUpgrade() {
    setScheduleUpgradesVisible(false);
  }

  function onUpdate() {
    return run(() =>
      ctx.upgradeWindowService
        .updateUpgradeWindowStart(selectedUpgradeWindowStart)
        .then(closeScheduleUpgrade)
    );
  }

  return {
    scheduleUpgradesVisible,
    showScheduleUpgrade,
    closeScheduleUpgrade,
    selectedUpgradeWindowStart,
    setSelectedUpgradeWindowStart,
    onUpdate,
    attempt,
  };
}
