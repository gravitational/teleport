import React from 'react';

import { Alert, Box, Flex, Indicator, Text, Link } from 'design';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';

import { useDevices, State } from './useDevices';

import { DeviceList } from './DeviceList';
import { EmptyList } from './EmptyList';

export const Container = () => {
  const state = useDevices();
  return <DeviceTrust {...state} />;
};

export const DeviceTrust = (props: State) => {
  let { attempt, items, fetchData, fetchStatus } = props;

  const isEmpty = items?.length === 0;
  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>Trusted Devices</FeatureHeaderTitle>
      </FeatureHeader>
      {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'success' && (
        <Flex alignItems="start">
          {isEmpty && (
            <Flex mt="4" width="100%" justifyContent="center">
              <EmptyList />
            </Flex>
          )}
          {!isEmpty && (
            <>
              <Box width="100%" mr="6" mb="4">
                <DeviceList
                  items={items}
                  fetchData={fetchData}
                  fetchStatus={fetchStatus}
                />
              </Box>

              <Box
                ml="auto"
                width="240px"
                color="text.primary"
                style={{ flexShrink: 0 }}
              >
                <Text typography="h6" mb={3} caps>
                  Register Trusted Device
                </Text>
                <Text typography="subtitle1" mb={3}>
                  Device Trust allows Teleport admins to enforce the use of
                  trusted devices. Resources protected by the device mode
                  "required" will enforce authenticated device access.
                </Text>
                <Text typography="subtitle1">
                  Please{' '}
                  <Link
                    color="text.primary"
                    href="https://goteleport.com/docs/access-controls/guides/device-trust/"
                    target="_blank"
                  >
                    view our documentation
                  </Link>{' '}
                  on how to get started with Device Trust.
                </Text>
              </Box>
            </>
          )}
        </Flex>
      )}
    </FeatureBox>
  );
};
