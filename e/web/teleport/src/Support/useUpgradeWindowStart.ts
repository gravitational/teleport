import { useEffect, useState } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';

import type { UpgradeWindowStartHour } from 'e-teleport/services/upgradeWindow';
import TeleportContextE from 'e-teleport/teleportContextE';

export function useUpgradeWindowStart(
  ctx: TeleportContextE,
  clusterId: string,
  isCloud: boolean
) {
  const { attempt: fetchWindowAttempt, run: fetchWindowRun } = useAttempt();
  const { attempt: updateWindowAttempt, run: updateWindowRun } = useAttempt();

  const [scheduleUpgradesVisible, setScheduleUpgradesVisible] = useState(false);
  const [selectedUpgradeWindowStartHour, setSelectedUpgradeWindowStartHour] =
    useState<UpgradeWindowStartHour>(8);

  useEffect(() => {
    if (!isCloud) {
      return;
    }

    fetchWindowRun(() =>
      ctx.upgradeWindowService
        .getUpgradeWindowStartHour(clusterId)
        .then(setSelectedUpgradeWindowStartHour)
    );
  }, [clusterId, isCloud, fetchWindowRun, ctx.upgradeWindowService]);

  function showScheduleUpgrade() {
    setScheduleUpgradesVisible(true);
  }

  function closeScheduleUpgrade() {
    setScheduleUpgradesVisible(false);
  }

  function onUpdate() {
    return updateWindowRun(() =>
      ctx.upgradeWindowService
        .updateUpgradeWindowStart(clusterId, selectedUpgradeWindowStartHour)
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
    fetchWindowAttempt,
    updateWindowAttempt,
  };
}
