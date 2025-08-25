import { Alert, Box, Flex, H3, Indicator, Link } from 'design';
import { P, P1, P2 } from 'design/Text/Text';

import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { EmptyList } from 'teleport/DeviceTrust/EmptyList';
import { CtaEvent } from 'teleport/services/userEvent';

import { DeviceList } from './DeviceList';
import { useDevices } from './useDevices';

export const DeviceTrust = () => {
  const props = useDevices();
  let {
    attempt,
    items,
    fetchData,
    fetchStatus,
    showTrustedDevicesCTA,
    missingPermissions,
  } = props;
  const canList = missingPermissions.length === 0;
  const isEmpty = items?.length === 0;

  return (
    <FeatureBox>
      {attempt.status === 'failed' && <Alert>{attempt.statusText}</Alert>}
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'success' && (
        <>
          {!canList && (
            <Alert kind="info" mt={4}>
              <Flex gap={2}>
                You do not have permission to access Trusted Devices. Missing
                role permissions:{' '}
                {missingPermissions.map(perm => (
                  <code key={perm}>{perm}</code>
                ))}
              </Flex>
            </Alert>
          )}
          {isEmpty && (
            <Flex alignItems="start">
              <EmptyList isEnterprise={true} />
            </Flex>
          )}
          {!isEmpty && (
            <>
              <FeatureHeader>
                <FeatureHeaderTitle>Trusted Devices</FeatureHeaderTitle>
              </FeatureHeader>
              <Flex>
                <Box width="100%" mr="6" mb="4" data-testid="devices-list">
                  <DeviceList
                    pagerPosition="top"
                    items={items}
                    fetchData={fetchData}
                    fetchStatus={fetchStatus}
                  />
                </Box>

                <Box
                  ml="auto"
                  width="240px"
                  color="text.main"
                  style={{ flexShrink: 0 }}
                >
                  <H3 mb={3}>Register Trusted Device</H3>
                  <P>
                    Trusted Devices can be{' '}
                    <Link
                      color="text.main"
                      href="https://goteleport.com/docs/identity-governance/device-trust/device-management/#register-a-trusted-device"
                      target="_blank"
                    >
                      registered with tctl
                    </Link>{' '}
                    or synced from either{' '}
                    <Link
                      color="text.main"
                      href="https://goteleport.com/docs/identity-governance/device-trust/jamf-integration/"
                      target="_blank"
                    >
                      Jamf&nbsp;Pro
                    </Link>{' '}
                    or{' '}
                    <Link
                      color="text.main"
                      href="https://goteleport.com/docs/identity-governance/device-trust/intune-integration/"
                      target="_blank"
                    >
                      Microsoft&nbsp;Intune
                    </Link>
                    .
                  </P>
                </Box>
              </Flex>
            </>
          )}
          {showTrustedDevicesCTA && <CallToAction />}
        </>
      )}
    </FeatureBox>
  );
};

const CallToAction = () => {
  return (
    <Flex
      data-testid="devices-cta"
      flexDirection="column"
      mt={3}
      justifyContent="end"
      alignItems="center"
    >
      <P2 color="text.slightlyMuted">
        <i>Your plan includes five free Trusted Devices.</i>
      </P2>
      <P1 mt={2}>
        Want additional devices?
        <ButtonLockedFeature
          width="176px"
          textLink={true}
          event={CtaEvent.CTA_ACCESS_LIST}
          pl={1}
        >
          Contact Sales
        </ButtonLockedFeature>
      </P1>
    </Flex>
  );
};
