/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { Link } from 'react-router-dom';

import {
  Box,
  ButtonPrimary,
  ButtonPrimaryBorder,
  Card,
  Flex,
  H1,
  H2,
  ResourceIcon,
  Text,
} from 'design';

import cfg from 'teleport/config';

export default function Empty(props: Props) {
  const { canCreate, clusterId, emptyStateInfo } = props;

  const { readOnly, title } = emptyStateInfo;

  // always show the welcome for enterprise users who have access to create an app
  if (!canCreate) {
    return (
      <Box
        p={8}
        mx="auto"
        maxWidth="664px"
        textAlign="center"
        color="text.main"
        borderRadius="12px"
      >
        <H1 mb="3">{readOnly.title}</H1>
        <Text>
          Either there are no {readOnly.resource} in the "
          <Text as="span" bold>
            {clusterId}
          </Text>
          " cluster, or your roles don't grant you access.
        </Text>
      </Box>
    );
  }

  const dividerHeight = '54px';

  return (
    <Box
      p={8}
      pt={5}
      as={Flex}
      width="100%"
      mx="auto"
      alignItems="center"
      justifyContent="center"
    >
      <Card p={4} width="780px">
        <Box mb={4} textAlign="center">
          <ResourceIcon name="server" mx="auto" mb={4} height="150px" />
          <H1>{title}</H1>
        </Box>
        <Flex>
          <Box p={4} flex="1" textAlign="left">
            <H2>Automatically Discover</H2>
            <Text mb={2}>
              Connect your AWS, Azure, or GCP accounts to automatically scan and
              import all resources.
            </Text>
            <ButtonPrimary
              as={Link}
              to={cfg.getIntegrationsEnrollRoute({ tags: ['cloud'] })}
              width="100%"
            >
              Connect Cloud Account
            </ButtonPrimary>
          </Box>

          <Flex
            width="48px"
            flexDirection="column"
            alignItems="center"
            justifyContent="center"
            flexShrink={0}
          >
            <Box width="1px" height={dividerHeight} bg="text.muted" />
            <Text my={2} color="text.muted">
              or
            </Text>
            <Box width="1px" height={dividerHeight} bg="text.muted" />
          </Flex>

          <Box p={4} flex="1" textAlign="left">
            <H2>Manually Enter</H2>
            <Text mb={2}>
              Browse and add individual servers, databases, or apps one at a
              time.
            </Text>
            <ButtonPrimaryBorder
              as={Link}
              to={{
                pathname: cfg.routes.discover,
                state: { entity: 'unified_resource' },
              }}
              width="100%"
            >
              Add New Resource
            </ButtonPrimaryBorder>
          </Box>
        </Flex>
      </Card>
    </Box>
  );
}

export type EmptyStateInfo = {
  readOnly: {
    title: string;
    resource: string;
  };
  title: string;
};

export type Props = {
  canCreate: boolean;
  clusterId: string;
  emptyStateInfo: EmptyStateInfo;
};
