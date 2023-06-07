/*
Copyright 2020-2021 Gravitational, Inc.

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
import { Text, Box, Flex, ButtonPrimary, Card } from 'design';
import * as Icons from 'design/Icon';

import { State as ResourceState } from 'teleport/components/useResources';

export default function EmptyList({ onCreate }: Props) {
  return (
    <Card maxWidth="700px" p={4} as={Flex} alignItems="center">
      <Icons.Github
        style={{ textAlign: 'center' }}
        fontSize="150px"
        css={`
          color: ${props => props.theme.colors.spotBackground[1]};
        `}
        mr={5}
      />
      <Box>
        <Text typography="h6" mb={3} caps>
          Create Your First GitHub Connector
        </Text>
        <Text typography="subtitle1" mb={3}>
          Authentication connectors allow Teleport to authenticate users via an
          external identity source such as Okta, Active Directory, GitHub, etc.
          This authentication method is frequently called single sign-on (SSO).
        </Text>
        <Text typography="subtitle1">
          Open Source Teleport supports only GitHub connectors. Please{' '}
          <Text
            as="a"
            color="text.main"
            href="https://goteleport.com/docs/setup/admin/github-sso/"
            target="_blank"
          >
            view our documentation
          </Text>{' '}
          on how to configure a GitHub connector.
        </Text>
        <ButtonPrimary onClick={onCreate} mt={4} width="240px">
          New GitHub Connector
        </ButtonPrimary>
      </Box>
    </Card>
  );
}

type Props = {
  onCreate: ResourceState['create'];
};
