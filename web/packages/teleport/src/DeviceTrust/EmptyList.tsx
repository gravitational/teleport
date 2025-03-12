/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';

import {
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  H1,
  H2,
  H3,
  P1,
  P2,
  ResourceIcon,
  Text,
} from 'design';
import Table from 'design/DataTable';
import {
  Cross,
  DeviceMobileCamera,
  FingerprintSimple,
  Password,
  UsbDrive,
} from 'design/Icon';
import {
  DetailsTab,
  FeatureContainer,
  FeatureSlider,
} from 'shared/components/EmptyState/EmptyState';
import { pluralize } from 'shared/utils/text';

import {
  renderDescCell,
  renderTimeCell,
} from 'teleport/Audit/EventList/EventList';
import renderTypeCell from 'teleport/Audit/EventList/EventTypeCell';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import cfg from 'teleport/config';
import { TrustedDevice } from 'teleport/DeviceTrust/types';
import { makeEvent } from 'teleport/services/audit';
import { CtaEvent } from 'teleport/services/userEvent';

import { DeviceList } from './DeviceList';

const maxWidth = '1270px';

export const EmptyList = ({
  isEnterprise = false,
}: {
  isEnterprise?: boolean;
}) => {
  const [currIndex, setCurrIndex] = useState(0);
  const [intervalId, setIntervalId] = useState<any>();

  function handleOnClick(clickedIndex: number) {
    clearInterval(intervalId);
    setCurrIndex(clickedIndex);
    setIntervalId(null);
  }

  useEffect(() => {
    const id = setInterval(() => {
      setCurrIndex(latestIndex => (latestIndex + 1) % 4);
    }, 3000);
    setIntervalId(id);
    return () => clearInterval(id);
  }, []);

  return (
    <Box mt={4} data-testid="devices-empty-state">
      <Box mb={3}>
        <H1 mb={3}>What are Trusted Devices?</H1>
        <Text css={{ maxWidth }}>
          Device trust reduces the attack surface by enforcing that only
          trusted, registered devices can access your Teleport cluster.
        </Text>
      </Box>
      <FeatureContainer py={2} pr={2}>
        <Box css={{ position: 'relative' }}>
          <FeatureSlider $currIndex={currIndex} />
          <DetailsTab
            active={currIndex === 0}
            isSliding={!!intervalId}
            onClick={() => handleOnClick(0)}
            title="Guarantee the provenance of the machines accessing your infrastructure."
            description="Teleport uses security devices - TPMs on Windows and Linux and secure enclaves on Macs to give every device a cryptographic identity."
          />
          <DetailsTab
            active={currIndex === 1}
            isSliding={!!intervalId}
            onClick={() => handleOnClick(1)}
            title="Reduce the attack surface from the entire internet to a limited fleet of clients."
            description="Make sure that only registered, cryptographically verified, and trusted devices can access your infrastructure."
          />
          <DetailsTab
            active={currIndex === 2}
            isSliding={!!intervalId}
            onClick={() => handleOnClick(2)}
            title="Tie audit events to the user AND machine accessing the system."
            description="Device trust maps the device identity to every audit log event, so you always know which device was used for each action."
          />
          <DetailsTab
            active={currIndex === 3}
            isSliding={!!intervalId}
            onClick={() => handleOnClick(3)}
            title="Integrates with your MDM"
            description="Auto-enroll and sync device registry from Jamf."
          />
        </Box>
        <Box>
          {currIndex === 0 && (
            <PreviewBox>
              <H2 mb={2}>Trusted Devices</H2>
              <List />
            </PreviewBox>
          )}
          {currIndex === 1 && (
            <FadedTable>
              <AccessDeniedCard />
            </FadedTable>
          )}
          {currIndex === 2 && (
            <PreviewBox>
              <H2 mb={2}>Audit Log</H2>
              <AuditList />
            </PreviewBox>
          )}
          {currIndex === 3 && (
            <FadedTable>
              <Flex flexDirection="column" justifyContent="center">
                <JamfCard margin="auto" mb={4}>
                  <ResourceIcon height="20px" width="20px" name="jamf" />
                  {/* purposefully "creating" the text ourselves to avoid having a light and dark logo just for text */}
                  <Text
                    css={`
                      font-size: 28px;
                      line-height: 30px;
                    `}
                    ml={3}
                  >
                    jamf
                  </Text>
                </JamfCard>
                <Flex justifyContent="center" gap={4}>
                  <IconCard>
                    <Password />
                  </IconCard>
                  <IconCard>
                    <UsbDrive />
                  </IconCard>
                  <IconCard>
                    <DeviceMobileCamera />
                  </IconCard>
                  <IconCard>
                    <FingerprintSimple />
                  </IconCard>
                </Flex>
              </Flex>
            </FadedTable>
          )}
        </Box>
      </FeatureContainer>
      {/* setting a max width here to keep it "in the center" with the content above instead of with the screen */}
      <Flex
        justifyContent="center"
        width="100%"
        maxWidth={maxWidth}
        textAlign="center"
        flexDirection="column"
        mt={6}
      >
        <Flex
          gap={3}
          width="100%"
          maxWidth={maxWidth}
          textAlign="center"
          justifyContent="center"
        >
          {isEnterprise ? (
            <>
              <ButtonPrimary
                width="280px"
                as={Link}
                to={cfg.getIntegrationEnrollRoute('jamf')}
                size="large"
              >
                Get Started with JAMF
              </ButtonPrimary>
              <ButtonSecondary
                as="a"
                href="https://goteleport.com/docs/admin-guides/access-controls/device-trust/jamf-integration/"
                target="_blank"
                width="280px"
                size="large"
              >
                See More Options in Our Docs
              </ButtonSecondary>
            </>
          ) : (
            <ButtonLockedFeature
              height="36px"
              width="500px"
              event={CtaEvent.CTA_TRUSTED_DEVICES}
            >
              Unlock Trusted Devices With Teleport Enterprise
            </ButtonLockedFeature>
          )}
        </Flex>
      </Flex>
    </Box>
  );
};

export const fakeItems: TrustedDevice[] = [
  {
    id: 'FWPGP915V',
    assetTag: 'FWPGP915V',
    osType: 'macOS',
    enrollStatus: 'enrolled',
    owner: 'mykel',
  },
  {
    id: 'M7XJR4GK8823',
    assetTag: 'M7XJR4GK8823',
    osType: 'Windows',
    enrollStatus: 'enrolled',
    owner: 'lila',
  },
  {
    id: 'L2FQZ9VH4466',
    assetTag: 'L2FQZ9VH4466',
    osType: 'Linux',
    enrollStatus: 'enrolled',
    owner: 'bart',
  },
  {
    id: 'N8EYW1DP7732',
    assetTag: 'N8EYW1DP7732',
    osType: 'Linux',
    enrollStatus: 'not enrolled',
    owner: 'rafao',
  },
  {
    id: 'K5BHP6CT5598',
    assetTag: 'K5BHP6CT5598',
    osType: 'Windows',
    enrollStatus: 'not enrolled',
    owner: 'gzz',
  },
  {
    id: 'Y3RSL7FJ2104',
    assetTag: 'Y3RSL7FJ2104',
    osType: 'macOS',
    enrollStatus: 'enrolled',
    owner: 'ryry',
  },
];

const List = () => {
  return (
    <DeviceList
      pagerPosition="top"
      items={fakeItems}
      fetchData={() => null}
      fetchStatus={'disabled'}
    />
  );
};

const auditEvents = [
  {
    cert_type: 'user',
    code: 'TC000I',
    event: 'cert.create',
    identity: {
      user: 'lisa',
    },
    time: '2024-02-04T19:43:23.529Z',
  },
  {
    cluster_name: 'im-a-cluster-name',
    code: 'TV006I',
    event: 'device.authenticate',
    success: true,
    time: '2024-02-04T19:43:22.529Z',
    uid: 'fa279611-91d8-47b5-9fad-b8ea3e5286e0',
    user: 'lisa',
  },
  {
    cert_type: 'user',
    code: 'TC000I',
    event: 'cert.create',
    identity: {
      user: 'isabelle',
    },
    time: '2024-02-04T19:43:21.529Z',
  },
  {
    cluster_name: 'zarq',
    code: 'T1016I',
    time: '2024-02-04T19:43:20.529Z',
    uid: '815bbcf4-fb05-4e08-917c-7259e9332d69',
    user: 'isabelle',
  },
].map(makeEvent);

const AuditList = () => {
  return (
    <Table
      data={auditEvents}
      isSearchable
      initialSort={{ key: 'time', dir: 'DESC' }}
      columns={[
        {
          key: 'codeDesc',
          headerText: 'Type',
          isSortable: true,
          render: ev => renderTypeCell(ev),
        },
        {
          key: 'message',
          headerText: 'Description',
          render: renderDescCell,
        },
        {
          key: 'time',
          headerText: 'Created (UTC)',
          isSortable: true,
          render: renderTimeCell,
        },
      ]}
      emptyText={'No Events Found'}
    />
  );
};

const FadedTable = ({ children }) => {
  return (
    <PreviewBox>
      <H2 mb={2}>Trusted Devices</H2>
      <List />
      <Flex
        css={`
          position: absolute;
          height: 100%;
          width: 100%;
          top: 50%;
          left: 50%;
          justify-content: center;
          align-items: center;
          transform: translate(-50%, -50%);
          backdrop-filter: blur(2px);
          border-radius: ${p => p.theme.radii[3]}px;
          background-color: rgba(0, 0, 0, 0.5);
        `}
      >
        {children}
      </Flex>
    </PreviewBox>
  );
};

const AccessDeniedCard = () => {
  return (
    <Flex
      css={`
        height: 160px;
        width: 330px;
        justify-content: center;
        align-items: center;
        flex-direction: column;
        border-radius: ${p => p.theme.radii[3]}px;
        background-color: ${props => props.theme.colors.levels.surface};
      `}
    >
      <Flex
        css={`
          border: 2px solid ${p => p.theme.colors.spotBackground[0]};
          position: relative;
          justify-content: center;
          align-items: center;
          padding: ${p => p.theme.space[2]}px;
          border-radius: ${p => p.theme.radii[3]}px;
        `}
      >
        <DeviceMobileCamera />
        <Flex
          css={`
            position: absolute;
            border-radius: 50%;
            justify-content: center;
            align-items: center;
            width: 24px;
            height: 24px;
            background: ${p => p.theme.colors.buttons.warning.default};
            bottom: -12px;
            right: -12px;
          `}
        >
          <Cross size="medium" color="text.primaryInverse" />
        </Flex>
      </Flex>
      <H3 mt={3}>Access Denied</H3>
      <Text>This device is not registered for access.</Text>
    </Flex>
  );
};

export function FeatureLimitBlurb({ limit = 1 }: { limit: number }) {
  if (limit === 0) {
    // unlimited access
    return null;
  }

  const listText = pluralize(limit, 'List');
  return (
    <Box
      mt={4}
      css={`
        text-align: center;
      `}
    >
      <P2 color="text.slightlyMuted">
        <i>
          Your current plan supports {limit} free Access {listText}.
        </i>
      </P2>
      <P1 mt={1}>
        Want additional Access Lists?{' '}
        <ButtonLockedFeature
          width="176px"
          textLink={true}
          event={CtaEvent.CTA_ACCESS_LIST}
          pl={1}
        >
          Contact Sales
        </ButtonLockedFeature>
      </P1>
    </Box>
  );
}

const PreviewBox = styled(Box)`
  margin-left: ${p => p.theme.space[5]}px;
  width: 675px;
  position: relative;
  padding: 12px;
  border-radius: ${p => p.theme.radii[3]}px;
`;

const JamfCard = styled(Flex)`
  justify-content: center;
  align-items: center;
  padding: ${p => p.theme.space[5]}px;
  border-radius: ${p => p.theme.radii[3]}px;
  background-color: ${p => p.theme.colors.levels.surface};
`;

const IconCard = styled(Flex)`
  justify-content: center;
  align-items: center;
  cursor: pointer;
  height: ${p => p.theme.space[5]}px;
  width: ${p => p.theme.space[5]}px;
  padding: ${p => p.theme.space[5]}px;
  border-radius: ${p => p.theme.radii[3]}px;
  background-color: ${p => p.theme.colors.levels.surface};
  color: ${p => p.theme.colors.text.main};
  &:hover {
    background-color: ${p => p.theme.colors.interactive.solid.primary.hover};
    color: ${p => p.theme.colors.text.primaryInverse};
  }
  transition: all 0.1s;
`;
