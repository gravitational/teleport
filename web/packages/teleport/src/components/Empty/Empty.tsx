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

import { Text, Box, Flex, ButtonPrimary, Link } from 'design';
import Card from 'design/Card';
import Image from 'design/Image';
import * as Icons from 'design/Icon';
import empty from './assets';

export default function Empty(props: Props) {
  const { canCreate, onClick, clusterId, emptyStateInfo } = props;

  const {
    title,
    description,
    buttonText,
    videoLink,
    readOnly,
  } = emptyStateInfo;

  // always show the welcome for enterprise users who have access to create an app
  if (!canCreate) {
    return (
      <Box
        p={8}
        mt={4}
        mx="auto"
        maxWidth="600px"
        textAlign="center"
        color="text.primary"
        bg="primary.light"
        borderRadius="12px"
      >
        <Text typography="h2" mb="3">
          {readOnly.title}
        </Text>
        <Text>
          {readOnly.message}
          <Text as="span" bold>
            {clusterId}
          </Text>
          " cluster.
        </Text>
      </Box>
    );
  }

  return (
    <Card
      p={4}
      as={Flex}
      maxWidth="900px"
      width="100%"
      mt={4}
      mx="auto"
      bg="primary.main"
    >
      <Flex
        as={Link}
        mr={4}
        maxWidth="296px"
        maxHeight="176px"
        bg="primary.dark"
        p={4}
        borderRadius={8}
        alignItems="center"
        justifyContent="center"
        style={{ position: 'relative' }}
        target="_blank"
        href={videoLink}
      >
        <Image width="220px" src={empty} />
        <Flex
          style={{ position: 'absolute' }}
          flexDirection="column"
          alignItems="center"
          mt={3}
        >
          <Icons.CirclePlay mb={3} fontSize="64px" />
          <Text color="text.primary" fontWeight={700}>
            WATCH THE QUICKSTART
          </Text>
        </Flex>
      </Flex>
      <Box>
        <Box mb={4}>
          <Text typography="h3" mb={2} fontWeight={700} fontSize={14}>
            {title}
          </Text>
          {description}
        </Box>
        <ButtonPrimary onClick={onClick} width="224px">
          {buttonText}
        </ButtonPrimary>
      </Box>
    </Card>
  );
}

export type EmptyStateInfo = {
  title: string;
  description: JSX.Element;
  buttonText: string;
  videoLink: string;
  readOnly: {
    title: string;
    message: string;
  };
};

export type Props = {
  canCreate: boolean;
  onClick(): void;
  clusterId: string;
  emptyStateInfo: EmptyStateInfo;
};
