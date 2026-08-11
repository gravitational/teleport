import { Box, Text, Mark } from 'design';
import { Info } from 'design/Alert';
import FieldInput from 'shared/components/FieldInput';
import { requiredField } from 'shared/components/Validation/rules';

import type { PluginEntraIdSyncIntervals } from 'teleport/services/integrations';

import { isZeroDuration } from './rules';

export function SyncIntervals({
  syncIntervals,
  onIntervalChange,
  disabled,
}: {
  syncIntervals: PluginEntraIdSyncIntervals;
  onIntervalChange: (PluginEntraIdSyncIntervals) => void;
  disabled: boolean;
}) {
  function handleFullInterval(value: string) {
    onIntervalChange({ ...syncIntervals, full: value });
  }

  function handleDeltaInterval(value: string) {
    onIntervalChange({ ...syncIntervals, delta: value });
  }

  const fullSyncTooltip = (
    <Text>
      Enter a Go duration string. Examples:
      <br />
      <Mark>30m</Mark>: run full sync every 30 minutes.
      <br />
      <Mark>1h</Mark>: run full sync every hour.
      <br />
      <Mark>0s</Mark>: disable full sync.
    </Text>
  );

  const deltaSyncTooltip = (
    <Text>
      Enter a Go duration string. Examples:
      <br />
      <Mark>2m</Mark>: run delta sync every 2 minutes.
      <br />
      <Mark>0s</Mark>: disable delta sync.
      <br />
    </Text>
  );

  return (
    <Box>
      <Text bold typography="subtitle1" mb={1}>
        Delta and Full Sync
      </Text>
      <Text mb={3}>
        Interval value must be a valid Go duration string. Examples:
        <br />
        <Mark>Delta=0s, Full=5m</Mark>: run full sync every 5 minutes. <br />
        <Mark>Delta=2m, Full=1h</Mark>: run delta sync every 2 minutes and full
        sync every hour.
        <br />
      </Text>
      <FieldInput
        label="Delta Sync Interval"
        width="540px"
        value={syncIntervals.delta}
        rule={requiredField(
          'Sync interval is required, use 0s to disable delta sync.'
        )}
        onChange={e => handleDeltaInterval(e.target.value)}
        placeholder="E.g., 1m, 2m, 10m etc."
        toolTipContent={deltaSyncTooltip}
        disabled={disabled}
      />
      <FieldInput
        label="Full Sync Interval"
        width="540px"
        value={syncIntervals.full}
        onChange={e => handleFullInterval(e.target.value)}
        rule={requiredField(
          'Sync interval is required, use 0s to disable full sync.'
        )}
        placeholder="E.g., 5m, 1h, 3h, etc."
        toolTipContent={fullSyncTooltip}
        disabled={disabled}
      />

      <ZeroIntervalsNote syncIntervals={syncIntervals} />
    </Box>
  );
}

function ZeroIntervalsNote({
  syncIntervals,
}: {
  syncIntervals: PluginEntraIdSyncIntervals;
}) {
  if (
    isZeroDuration(syncIntervals.delta) &&
    isZeroDuration(syncIntervals.full)
  ) {
    return (
      <Info>
        You haven't configured full or delta sync interval. The Entra ID service
        will default to running a full sync every five minutes.
      </Info>
    );
  }
  return null;
}
