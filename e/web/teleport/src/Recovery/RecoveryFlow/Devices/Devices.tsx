import { useMemo } from 'react';

import { Box, ButtonPrimary, Card, Indicator, P3 } from 'design';
import { OutlineDanger } from 'design/Alert/Alert';
import { StepHeader } from 'design/StepSlider';

import MfaDeviceList, { RemoveDialog } from 'teleport/components/MfaDeviceList';
import MfaService from 'teleport/services/mfa';

import useDevices, { Props, State } from './useDevices';

export default function Container(props: Props) {
  const mfaService = useMemo(() => new MfaService(), []);
  const state = useDevices(props, mfaService);
  return <Devices {...state} />;
}

export function Devices({
  attempt,
  devices,
  removeDevice,
  onNext,
  closeDialog,
  deviceToRemove,
  setDeviceToRemove,
}: State) {
  const mostRecentDevice = useMemo(
    () =>
      devices.sort((a, b) => {
        return b.registeredDate.getTime() - a.registeredDate.getTime();
      })[0],
    [devices]
  );

  return (
    <Card mx="auto" width="604px" p={4}>
      <Box mb={4}>
        <StepHeader
          stepIndex={2}
          flowLength={4}
          title="Device Successfully Enrolled"
        />
      </Box>

      <P3 mb={3} textAlign="center" color="text.slightlyMuted">
        Take a look at your enrolled devices below and remove any that you don't
        need anymore.
      </P3>
      <Box mb={4}>
        {attempt.status === 'processing' && (
          <Box textAlign="center">
            <Indicator />
          </Box>
        )}
        {attempt.status === 'failed' && (
          <OutlineDanger m={0}>{attempt.statusText}</OutlineDanger>
        )}
        {attempt.status === 'success' && (
          <MfaDeviceList
            devices={devices}
            remove={setDeviceToRemove}
            mostRecentDevice={mostRecentDevice}
            style={{ overflow: 'hidden', borderRadius: '8px' }}
          />
        )}
      </Box>
      <ButtonPrimary size="large" width="100%" type="submit" onClick={onNext}>
        Continue
      </ButtonPrimary>

      {deviceToRemove && (
        <RemoveDialog
          onClose={closeDialog}
          onRemove={removeDevice}
          name={deviceToRemove.name}
        />
      )}
    </Card>
  );
}
