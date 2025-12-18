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
  Flex,
  H1,
  ResourceIcon,
  Text,
} from 'design';

import cfg from 'teleport/config';

export default function Empty(props: Props) {
  const { canCreate, clusterId, emptyStateInfo } = props;

  const { byline, readOnly, title } = emptyStateInfo;

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
      <Box maxWidth={600}>
        <Box mb={3} textAlign="center">
          <ResourceIcon name="server" mx="auto" mb={4} height="160px" />
          <H1>{title}</H1>
          <Text fontWeight={400} fontSize={14} style={{ opacity: '0.6' }}>
            {byline}
          </Text>
        </Box>
        <Box textAlign="center">
          <Text fontWeight={400} fontSize={14} style={{ opacity: '0.6' }}>
            Automatically discover your Cloud Infrastructure resources
          </Text>
        </Box>
        <Box mb={3} textAlign="center">
          <ButtonPrimary
            as={Link}
            to={cfg.getIntegrationsEnrollRoute({ tags: ['cloud'] })}
            width="224px"
          >
            Connect a Cloud Account
          </ButtonPrimary>
        </Box>
        <Box textAlign="center">
          <Text fontWeight={400} fontSize={14} style={{ opacity: '0.6' }}>
            Or add individual resources
          </Text>
        </Box>
        <Box textAlign="center">
          <ButtonPrimaryBorder
            as={Link}
            to={{
              pathname: `${cfg.routes.discover}`,
              state: {
                entity: 'unified_resource',
              },
            }}
            width="224px"
          >
            Add a Resource
          </ButtonPrimaryBorder>
        </Box>
      </Box>
    </Box>
  );
}

export type EmptyStateInfo = {
  byline: string;
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
