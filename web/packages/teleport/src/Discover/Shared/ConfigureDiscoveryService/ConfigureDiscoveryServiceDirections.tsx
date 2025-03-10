/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
import styled from 'styled-components';

import { Box, Flex, H3, Input, Mark, Subtitle3, Text } from 'design';
import { P } from 'design/Text/Text';
import { IconTooltip } from 'design/Tooltip';

import { Tabs } from 'teleport/components/Tabs';
import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';

const discoveryGroupToolTip = `Discovery group name is used to group discovered resources into different sets. \
This parameter is used to prevent Discovery Agents watching different sets of cloud resources from \
colliding against each other and deleting resources created by another services.`;

const discoveryServiceToolTip = `The Discovery Service is responsible for watching your \
cloud provider and checking if there are any new resources or if there have been any \
modifications to previously discovered resources.`;

export const ConfigureDiscoveryServiceDirections = ({
  clusterPublicUrl,
  discoveryGroupName,
  setDiscoveryGroupName,
  showSubHeader = true,
}: {
  clusterPublicUrl: string;
  discoveryGroupName: string;
  setDiscoveryGroupName(n: string): void;
  showSubHeader?: boolean;
}) => {
  const yamlContent = `version: v3
teleport:
  join_params:
    token_name: "<YOUR_JOIN_TOKEN_FROM_STEP_1>"
    method: token
  proxy_server: "${clusterPublicUrl}"
auth_service:
  enabled: off
proxy_service:
  enabled: off
ssh_service:
  enabled: off
discovery_service:
  enabled: "yes"
  discovery_group: "${discoveryGroupName}"`;

  return (
    <Box mt={2}>
      {showSubHeader && (
        <>
          <Flex alignItems="center">
            <Text>
              Auto-enrolling requires you to configure a{' '}
              <Mark>Discovery Service</Mark>
            </Text>
            <IconTooltip children={discoveryServiceToolTip} />
          </Flex>
          <br />
        </>
      )}
      <StyledBox mb={5}>
        <header>
          <H3>Step 1</H3>
          <Subtitle3 mb={2}> Create a Join Token</Subtitle3>
        </header>
        <P mb={2}>
          Run the following command against your Teleport Auth Service and save
          it in <Mark>/tmp/token</Mark> on the host that will run the Discovery
          Service.
        </P>
        <TextSelectCopyMulti
          lines={[
            {
              text: `tctl tokens add --type=discovery`,
            },
          ]}
        />
      </StyledBox>
      <StyledBox mb={5}>
        <Flex alignItems="center">
          <header>
            <H3>Step 2</H3>
            <Subtitle3>
              Define a Discovery Group name{' '}
              <IconTooltip children={discoveryGroupToolTip} />
            </Subtitle3>
          </header>
        </Flex>
        <Box mt={3} width="260px">
          <Input
            value={discoveryGroupName}
            onChange={e => setDiscoveryGroupName(e.target.value)}
            hasError={discoveryGroupName.length == 0}
          />
        </Box>
      </StyledBox>
      <StyledBox mb={5}>
        <header>
          <H3>Step 3</H3>
          <Subtitle3 mb={2}>Create a teleport.yaml file</Subtitle3>
        </header>
        <P mb={2}>
          Use this template to create a <Mark>teleport.yaml</Mark> on the host
          that will run the Discovery Service.
        </P>
        <TextSelectCopyMulti lines={[{ text: yamlContent }]} bash={false} />
      </StyledBox>
      <StyledBox mb={5}>
        <header>
          <H3>Step 4</H3>
          <Subtitle3 mb={2}>Start Discovery Service</Subtitle3>
        </header>
        <P mb={2}>
          Configure the Discovery Service to start automatically when the host
          boots up by creating a systemd service for it. The instructions depend
          on how you installed the Discovery Service.
        </P>
        <Tabs
          tabs={[
            {
              title: 'Package Manager',
              content: (
                <Box px={2} pb={2}>
                  <Text mb={2}>
                    On the host where you will run the Discovery Service, enable
                    and start Teleport:
                  </Text>
                  <TextSelectCopyMulti
                    lines={[
                      {
                        text: `sudo systemctl enable teleport`,
                      },
                      {
                        text: `sudo systemctl start teleport`,
                      },
                    ]}
                  />
                </Box>
              ),
            },
            {
              title: `TAR Archive`,
              content: (
                <Box px={2} pb={2}>
                  <Text mb={2}>
                    On the host where you will run the Discovery Service, create
                    a systemd service configuration for Teleport, enable the
                    Teleport service, and start Teleport:
                  </Text>
                  <TextSelectCopyMulti
                    lines={[
                      {
                        text: `sudo teleport install systemd -o /etc/systemd/system/teleport.service`,
                      },
                      {
                        text: `sudo systemctl enable teleport`,
                      },
                      {
                        text: `sudo systemctl start teleport`,
                      },
                    ]}
                  />
                </Box>
              ),
            },
          ]}
        />
        <P mt={2}>
          You can check the status of the Discovery Service with{' '}
          <Mark>systemctl status teleport</Mark> and view its logs with{' '}
          <Mark>journalctl -fu teleport</Mark>.
        </P>
      </StyledBox>
    </Box>
  );
};

const StyledBox = styled(Box)`
  max-width: 1000px;
  background-color: ${props => props.theme.colors.spotBackground[0]};
  padding: ${props => `${props.theme.space[3]}px`};
  border-radius: ${props => `${props.theme.space[2]}px`};
`;
