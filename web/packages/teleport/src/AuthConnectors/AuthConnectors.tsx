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

import { Alert, Box, ButtonPrimary, Flex, Indicator, Link, Text } from 'design';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import ResourceEditor from 'teleport/components/ResourceEditor';
import useResources from 'teleport/components/useResources';

import EmptyList from './EmptyList';
import ConnectorList from './ConnectorList';
import DeleteConnectorDialog from './DeleteConnectorDialog';
import useAuthConnectors, { State } from './useAuthConnectors';
import templates from './templates';

export default function Container() {
  const state = useAuthConnectors();
  return <AuthConnectors {...state} />;
}

export function AuthConnectors(props: State) {
  const { attempt, items, remove, save } = props;
  const isEmpty = items.length === 0;
  const resources = useResources(items, templates);

  const title =
    resources.status === 'creating'
      ? 'Creating a new github connector'
      : 'Editing github connector';

  function handleOnSave(content: string) {
    const name = resources.item.name;
    const isNew = resources.status === 'creating';
    return save(name, content, isNew);
  }

  return (
    <FeatureBox>
      <FeatureHeader
        css={`
          @media screen and (max-width: 800px) {
            flex-direction: column;
            height: auto;
            gap: 10px;
            margin: 0 0 10px 0;
            padding-bottom: 10px;
            justify-content: center;
          }
        `}
      >
        <FeatureHeaderTitle>Auth Connectors</FeatureHeaderTitle>
        <ButtonPrimary width="240px" onClick={() => resources.create('github')}>
          New GitHub Connector
        </ButtonPrimary>
      </FeatureHeader>
      {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
      {attempt.status === 'processing' && (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      )}
      {attempt.status === 'success' && (
        <Flex alignItems="start">
          {isEmpty && (
            <Flex width="100%" justifyContent="center">
              <EmptyList onCreate={() => resources.create('github')} />
            </Flex>
          )}
          <>
            <ConnectorList
              items={items}
              onEdit={resources.edit}
              onDelete={resources.remove}
            />
            <Box
              ml="4"
              width="240px"
              color="text.main"
              style={{ flexShrink: 0 }}
              css={`
                @media screen and (max-width: 1000px) {
                  display: none;
                } ;
              `}
            >
              <Text typography="h6" mb={3} caps>
                Auth Connectors
              </Text>
              <Text typography="subtitle1" mb={3}>
                Auth connectors allow Teleport to authenticate users via an
                external identity source such as Okta, Active Directory, GitHub,
                etc. This authentication method is commonly known as single
                sign-on (SSO).
              </Text>
              <Text typography="subtitle1" mb={2}>
                Please{' '}
                <Link
                  color="text.main"
                  // This URL is the OSS documentation for auth connectors
                  href="https://goteleport.com/docs/setup/admin/github-sso/"
                  target="_blank"
                >
                  view our documentation
                </Link>{' '}
                on how to configure a GitHub connector.
              </Text>
            </Box>
          </>
        </Flex>
      )}
      {(resources.status === 'creating' || resources.status === 'editing') && (
        <ResourceEditor
          title={title}
          onSave={handleOnSave}
          text={resources.item.content}
          name={resources.item.name}
          isNew={resources.status === 'creating'}
          onClose={resources.disregard}
        />
      )}
      {resources.status === 'removing' && (
        <DeleteConnectorDialog
          name={resources.item.name}
          onClose={resources.disregard}
          onDelete={() => remove(resources.item.name)}
        />
      )}
    </FeatureBox>
  );
}
