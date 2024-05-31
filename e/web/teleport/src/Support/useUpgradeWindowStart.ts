import { useState, useEffect } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';

import TeleportContextE from 'e-teleport/teleportContextE';
import cfg from 'e-teleport/config';

import type { UpgradeWindowStartHour } from 'e-teleport/services/upgradeWindow';

export function useUpgradeWindowStart(ctx: TeleportContextE) {
  const { attempt, run } = useAttempt();

  const [scheduleUpgradesVisible, setScheduleUpgradesVisible] = useState(false);
  const [selectedUpgradeWindowStartHour, setSelectedUpgradeWindowStartHour] =
    useState<UpgradeWindowStartHour>(8);

  useEffect(() => {
    if (!cfg.oss.isCloud) {
      return;
    }

    run(() =>
      ctx.upgradeWindowService
        .getUpgradeWindowStartHour()
        .then(setSelectedUpgradeWindowStartHour)
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
        .updateUpgradeWindowStart(selectedUpgradeWindowStartHour)
        .then(closeScheduleUpgrade)
    );
  }

  return {
    scheduleUpgradesVisible,
    showScheduleUpgrade,
    closeScheduleUpgrade,
    selectedUpgradeWindowStart: selectedUpgradeWindowStartHour,
    setSelectedUpgradeWindowStart: setSelectedUpgradeWindowStartHour,
    onUpdate,
    attempt,
  };
}
