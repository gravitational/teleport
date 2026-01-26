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

import { Link } from 'react-router';

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

  const showCloud = cfg.isCloud;

  return (
    <Box p={8} pt={5} width="100%">
      <Card
        p={4}
        maxWidth={showCloud ? '780px' : '390px'}
        minWidth={showCloud ? '593px' : '390px'}
        mx="auto"
      >
        <Box mb={showCloud ? 4 : 2} textAlign="center">
          <ResourceIcon name="server" mx="auto" mb={4} height="150px" />
          <H1>{title}</H1>
        </Box>
        <Flex>
          {showCloud && (
            <Pane
              title="Automatically Discover"
              text="Use Terraform to connect your AWS, Azure, or GCP accounts to Teleport and automatically discover your resources."
              button={
                <ButtonPrimary
                  as={Link}
                  to={cfg.getIntegrationsEnrollRoute({ tags: ['terraform'] })}
                  width="100%"
                >
                  Connect Cloud Account
                </ButtonPrimary>
              }
            />
          )}

          {showCloud && <Divider />}

          <Pane
            title={showCloud ? 'Manually Enter' : undefined}
            text="Browse and add individual servers, databases, or apps one at a time."
            button={
              <ButtonPrimaryBorder
                as={Link}
                to={cfg.routes.discover}
                state={{ entity: 'unified_resource' }}
                width="100%"
              >
                Enroll New Resource
              </ButtonPrimaryBorder>
            }
          />
        </Flex>
      </Card>
    </Box>
  );
}

function Pane({ title, text, button }) {
  return (
    <Flex p={4} flex={1} textAlign="left" flexDirection="column">
      {title && <H2>{title}</H2>}
      <Text mb={2}>{text}</Text>
      <Box mt="auto">{button}</Box>
    </Flex>
  );
}

function Divider() {
  const height = '54px';
  return (
    <Flex
      width="48px"
      flexDirection="column"
      alignItems="center"
      justifyContent="center"
      flexShrink={0}
    >
      <Box width="1px" height={height} bg="text.muted" />
      <Text my={2} color="text.muted">
        or
      </Text>
      <Box width="1px" height={height} bg="text.muted" />
    </Flex>
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
