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

import { Link as InternalLink } from 'react-router-dom';

import {
  Box,
  ButtonBorder,
  ButtonPrimary,
  Flex,
  H1,
  ResourceIcon,
  Text,
} from 'design';
import { pluralize } from 'shared/utils/text';

import cfg from 'teleport/config';
import { ResourceAccessKind } from 'teleport/Roles/RoleEditor/StandardEditor/standardmodel';

type EmptyResourceKind = ResourceAccessKind | 'awsIc';

type Base = {
  canCreate: boolean;
  clusterId: string;
};

type Custom = Base & {
  emptyStateInfo: EmptyStateInfo;
  kind?: never;
};

type SingleResource = Base & {
  kind: ResourceAccessKind | 'awsIc';
  emptyStateInfo?: never;
};

export default function Empty(props: Custom | SingleResource) {
  const { canCreate, clusterId } = props;

  let emptyStateInfo: EmptyStateInfo;
  let resourceKind: EmptyResourceKind;
  if (isCustomType(props)) {
    emptyStateInfo = props.emptyStateInfo;
  } else {
    emptyStateInfo = getEmptyStateInfo(props.kind);
    resourceKind = props.kind;
  }

  const { byline, docsURL, readOnly, title } = emptyStateInfo;

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
        <Box mb={4} textAlign="center">
          <ResourceIcon name="server" mx="auto" mb={4} height="160px" />
          <H1 mb={2}>{title}</H1>
          <Text fontWeight={400} fontSize={14} style={{ opacity: '0.6' }}>
            {byline}
          </Text>
        </Box>
        <Box textAlign="center">
          {/* Git server is created from adding a github integration which is
          different from discover */}
          {resourceKind === 'git_server' ? (
            <ButtonPrimary
              as={InternalLink}
              width="224px"
              to={{
                pathname: cfg.routes.integrationEnroll,
                search: 'github repo',
              }}
            >
              Add GitHub Repository Access
            </ButtonPrimary>
          ) : (
            <ButtonPrimary
              as={InternalLink}
              width="224px"
              textTransform="none"
              to={{
                pathname:
                  resourceKind === 'awsIc'
                    ? cfg.getIntegrationEnrollRoute('aws-identity-center')
                    : cfg.routes.discover,
                state: {
                  searchKeywords:
                    resourceKind && resourceKind !== 'awsIc'
                      ? getResourceSearchKeywords(resourceKind)
                      : '',
                },
              }}
            >
              Add {resourceKind === 'awsIc' ? 'Integration' : 'Resource'}
            </ButtonPrimary>
          )}
          {docsURL && (
            <ButtonBorder
              textTransform="none"
              size="medium"
              as="a"
              href={docsURL}
              target="_blank"
              width="224px"
              ml={4}
              rel="noreferrer"
            >
              View Documentation
            </ButtonBorder>
          )}
        </Box>
      </Box>
    </Box>
  );
}

export type EmptyStateInfo = {
  byline: string;
  docsURL?: string;
  readOnly: {
    title: string;
    resource: string;
  };
  title: string;
};

function isCustomType(prop: Custom | SingleResource): prop is Custom {
  return (prop as Custom).emptyStateInfo !== undefined;
}

function getEmptyStateTitleAndByline(resource: string) {
  const baseInfo: EmptyStateInfo = {
    title: '',
    byline: '',
    readOnly: {
      title: 'No Resources Found',
      resource: 'resources',
    },
  };

  return {
    ...baseInfo,
    title: `Add your first ${resource} to Teleport`,
    byline: `Connect ${pluralize(0, resource)} from our integrations catalog.`,
  };
}

function getEmptyStateInfo(kind: EmptyResourceKind): EmptyStateInfo {
  switch (kind) {
    case 'awsIc':
      return {
        title: 'Integrate with AWS IAM Identity Center',
        byline: `Connect your AWS IAM Identity Center to Teleport`,
        readOnly: {
          title: 'No Resources Found',
          resource: 'resources',
        },
      };
    case 'app':
      return getEmptyStateTitleAndByline('application');
    case 'db':
      return getEmptyStateTitleAndByline('database');
    case 'git_server':
      return getEmptyStateTitleAndByline('Git server');
    case 'kube_cluster':
      return getEmptyStateTitleAndByline('Kubernetes cluster');
    case 'node':
      return getEmptyStateTitleAndByline('SSH server');
    case 'windows_desktop':
      return getEmptyStateTitleAndByline('Windows desktop');
    default:
      kind satisfies never;
  }
}

function getResourceSearchKeywords(kind: ResourceAccessKind) {
  switch (kind) {
    case 'app':
      return 'application';
    case 'db':
      return 'database';
    case 'git_server':
      return 'git';
    case 'kube_cluster':
      return 'kube';
    case 'node':
      return 'server';
    case 'windows_desktop':
      return 'windows desktop';
  }
}
