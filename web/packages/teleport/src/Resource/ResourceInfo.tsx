import React, { PropsWithChildren } from 'react';
import styled from 'styled-components';

import { Box, Card, Flex, ResourceIcon, Text } from 'design';
import { UnifiedResource } from 'teleport/services/agents';
import {
  formatNodeSubKind,
  guessAppIcon,
} from 'shared/components/UnifiedResources/shared/viewItemsFactory';
import { ResourceIconName } from 'design/ResourceIcon';
import { ResourceActionButton } from 'teleport/UnifiedResources/ResourceActionButton';
// import { ArrowBack } from 'design/Icon';

export function ResourceInfo({
  resource,
  LockButton,
}: {
  resource: UnifiedResource;
  LockButton: () => JSX.Element;
}) {
  // const history = useHistory();

  let name: string;
  let hostname: string;
  let subKind: string;
  let id: string;
  let type: string;
  let address: string;
  let iconName: ResourceIconName;

  switch (resource.kind) {
    case 'app':
      name = resource.friendlyName || resource.name;
      type = 'Application';
      id = resource.name;
      address = resource.uri;
      iconName = guessAppIcon(resource);
      break;
    case 'node':
      hostname = resource.hostname;
      type = 'Node';
      subKind = formatNodeSubKind(resource.subKind);
      id = resource.id;
      address = resource.addr;
      iconName = 'Server';
      break;
  }

  return (
    <DataContainer>
      <Flex>
        <Box width="100%">
          <Flex
            alignItems="center"
            mb={3}
            width="100%"
            justifyContent="space-between"
          >
            <Flex alignItems="flex-start">
              {/* <ArrowBack
                onClick={() => history.goBack()}
                size="large"
                mr={2}
                title="Go Back"
                style={{ cursor: 'pointer', textDecoration: 'none' }}
              /> */}
              {iconName && (
                <ResourceIcon
                  name={iconName}
                  css={`
                    height: 24px;
                    width: 24px;
                    margin-right: ${props => props.theme.space[1]}px;
                  `}
                />
              )}
              <Text typography="h4" style={{ lineHeight: '24px' }}>
                Resource Details
              </Text>
            </Flex>
            <Flex flexDirection="column" gap={2}>
              <ResourceActionButton resource={resource} />
              <LockButton />
            </Flex>
          </Flex>
          {name && <DataItem title="Name" data={name} />}
          {hostname && <DataItem title="Hostname" data={hostname} />}
          {id && <DataItem title="ID" data={id} />}
          {type && <DataItem title="Type" data={type} />}
          {subKind && <DataItem title="Sub-kind" data={subKind} />}
        </Box>
      </Flex>
    </DataContainer>
  );
}

const DataContainer: React.FC<PropsWithChildren> = ({ children }) => (
  <StyledDataContainer mt={4} borderRadius={3} px={5} py={4}>
    {children}
  </StyledDataContainer>
);

export const DataItem = ({ title = '', data = null }) => (
  <Flex mb={3}>
    <Text typography="body2" bold style={{ width: '130px' }}>
      {title}:
    </Text>
    <Text typography="body2">{data}</Text>
  </Flex>
);

const StyledDataContainer = styled(Box)`
  border: 1px solid ${props => props.theme.colors.spotBackground[1]};
`;
