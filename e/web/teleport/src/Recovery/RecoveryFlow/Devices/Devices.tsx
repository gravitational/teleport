import React, { useMemo } from 'react';
import { Card, Text, Box, ButtonPrimary, Indicator } from 'design';
import { Danger } from 'design/Alert';
import MfaService from 'teleport/services/mfa';
import MfaDeviceList, { RemoveDialog } from 'teleport/components/MfaDeviceList';
import useDevices, { State, Props } from './useDevices';

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
    <Card bg="primary.light" mx="auto" width="604px">
      <Text typography="h3" pt={5} textAlign="center" color="light">
        Device Successfully Enrolled
      </Text>
      <Text textAlign="center" color="text.secondary">
        Step 3 of 4
      </Text>
      <Box p={5}>
        <Text
          typography="body2"
          mb={3}
          textAlign="center"
          color="text.secondary"
        >
          Take a look at your enrolled devices below and remove any that you
          don't need anymore.
        </Text>
        <Box>
          {attempt.status === 'processing' && (
            <Box textAlign="center">
              <Indicator />
            </Box>
          )}
          {attempt.status === 'failed' && (
            <Danger m={0}>{attempt.statusText}</Danger>
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
        <ButtonPrimary
          mt={6}
          size="large"
          width="100%"
          type="submit"
          onClick={onNext}
        >
          Continue
        </ButtonPrimary>
      </Box>
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
