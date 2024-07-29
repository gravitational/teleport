import React from 'react';

import { Alert, Box, Flex, Indicator, Link, H3, P2, P1 } from 'design';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';

import { CtaEvent } from 'teleport/services/userEvent';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';

import { P } from 'design/Text/Text';

import { useDevices } from './useDevices';

import { DeviceList } from './DeviceList';
import { EmptyList } from './EmptyList';

export const deviceTrustDocUrl =
  'https://goteleport.com/docs/access-controls/guides/device-trust';

export const DeviceTrust = () => {
  const props = useDevices();
  let { attempt, items, fetchData, fetchStatus, showTrustedDevicesCTA } = props;
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
            <Flex
              mt="4"
              flexDirection="column"
              width="100%"
              alignItems="center"
            >
              <EmptyList />
              {showTrustedDevicesCTA && <CallToAction align="center" />}
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
                {showTrustedDevicesCTA && <CallToAction align="end" />}
              </Box>

              <Box
                ml="auto"
                width="240px"
                color="text.main"
                style={{ flexShrink: 0 }}
              >
                <H3 mb={3}>Register Trusted Device</H3>
                <P>
                  Trusted Devices can be registered manually using{' '}
                  <Link
                    color="text.main"
                    href="https://goteleport.com/docs/access-controls/device-trust/guide/?scope=enterprise#step-12-register-a-trusted-device"
                    target="_blank"
                  >
                    tctl client
                  </Link>{' '}
                  or synced automatically from MDM services like{' '}
                  <Link
                    color="text.main"
                    href="https://goteleport.com/docs/access-controls/device-trust/jamf-integration/?scope=enterprise"
                    target="_blank"
                  >
                    Jamf
                  </Link>
                  .
                </P>
              </Box>
            </>
          )}
        </Flex>
      )}
    </FeatureBox>
  );
};

const CallToAction = (props: CTAProps) => {
  return (
    <Flex
      flexDirection="column"
      mt={3}
      justifyContent="end"
      alignItems={props.align}
    >
      <P2 color="text.slightlyMuted">
        <i>Your plan includes five free Trusted Devices.</i>
      </P2>
      <P1 mt={1} mb={2}>
        Want additional devices?
      </P1>
      <ButtonLockedFeature
        width="176px"
        noIcon
        event={CtaEvent.CTA_TRUSTED_DEVICES}
      >
        Contact Sales
      </ButtonLockedFeature>
    </Flex>
  );
};

type CTAProps = {
  align: 'center' | 'end';
};
