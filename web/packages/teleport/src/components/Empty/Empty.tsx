/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { Link } from 'react-router-dom';

import { Text, Box, Flex, ButtonPrimary, ButtonOutlined } from 'design';
import Image from 'design/Image';

import cfg from 'teleport/config';

import application from './assets/appplication.png';
import database from './assets/database.png';
import desktop from './assets/desktop.png';
import stack from './assets/stack.png';

type ResourceType =
  | 'application'
  | 'database'
  | 'desktop'
  | 'kubernetes'
  | 'server';

function getAccentImage(resourceType: ResourceType): string {
  const accentImages = {
    application: application,
    database: database,
    desktop: desktop,
    kubernetes: stack,
    server: stack,
  };
  return accentImages[resourceType];
}

export default function Empty(props: Props) {
  const { canCreate, clusterId, emptyStateInfo } = props;

  const { byline, docsURL, resourceType, readOnly, title } = emptyStateInfo;

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
        <Text typography="h2" mb="3">
          {readOnly.title}
        </Text>
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
          <Image
            src={getAccentImage(resourceType)}
            ml="auto"
            mr="auto"
            mb={4}
            height="160px"
          />
          <Text typography="h5" mb={2} fontWeight={700} fontSize={24}>
            {title}
          </Text>
          <Text fontWeight={400} fontSize={14} style={{ opacity: '0.6' }}>
            {byline}
          </Text>
        </Box>
        <Box textAlign="center">
          <Link
            to={{
              pathname: `${cfg.routes.root}/discover`,
              state: { entity: resourceType },
            }}
            style={{ textDecoration: 'none' }}
          >
            <ButtonPrimary width="224px">Add {resourceType}</ButtonPrimary>
          </Link>
          <ButtonOutlined
            size="medium"
            as="a"
            href={docsURL}
            target="_blank"
            width="224px"
            ml={4}
            rel="noreferrer"
          >
            View Documentation
          </ButtonOutlined>
        </Box>
      </Box>
    </Box>
  );
}

export type EmptyStateInfo = {
  byline: string;
  docsURL: string;
  resourceType: ResourceType;
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
