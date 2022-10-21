/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { Flex, Text } from 'design';
import TextEditor from 'shared/components/TextEditor';

import { kubeAccessRW, kubeAccessRO } from '../templates';

export function AccessInfo({ accessKind, resourceName, traitDesc }: Props) {
  switch (accessKind) {
    case 'ssoUserAndNoTraits':
      return (
        <>
          <Info>
            You don’t have any {resourceName} {traitDesc} defined.
            <br />
            Please ask your Teleport administrator to update your role and add
            the required {resourceName} {traitDesc}.
          </Info>
          <YamlReader resource={resourceName} userAccessReadOnly={true} />
        </>
      );
    case 'noAccessAndNoTraits':
      return (
        <>
          <Info>
            You don’t have {resourceName} access.
            <br />
            Please ask your Teleport administrator to update your role:
          </Info>
          <YamlReader resource={resourceName} />
        </>
      );
    case 'noAccessButHasTraits':
      return (
        <>
          <Info>
            You don't have permission to add new {resourceName} {traitDesc}.
            <br />
            If you don't see the {resourceName} {traitDesc} that you require,
            please ask your Teleport administrator to update your role:
          </Info>
          <YamlReader resource={resourceName} />
        </>
      );
    case 'ssoUserButHasTraits':
      return (
        <>
          <Info>
            SSO users are not able to add new {resourceName} {traitDesc}.
            <br />
            If you don't see the {resourceName} {traitDesc} that you require,
            please ask your Teleport administrator to update your role:
          </Info>
          <YamlReader resource={resourceName} userAccessReadOnly={true} />
        </>
      );
  }
}

function YamlReader({
  resource,
  userAccessReadOnly,
}: {
  resource: ResourceName;
  userAccessReadOnly?: boolean;
}) {
  switch (resource) {
    case 'Kubernetes':
      if (userAccessReadOnly) {
        return (
          <Flex minHeight="215px" mt={3}>
            <ReadOnlyYamlEditor content={kubeAccessRO} />
          </Flex>
        );
      }
      return (
        <Flex minHeight="370px" mt={3}>
          <ReadOnlyYamlEditor content={kubeAccessRW} />
        </Flex>
      );
  }
}

const Info = ({ children }: { children: React.ReactNode }) => (
  <Text mt={4} width="100px">
    {children}
  </Text>
);

const ReadOnlyYamlEditor = ({ content }: { content: string }) => {
  return <TextEditor readOnly={true} data={[{ content, type: 'yaml' }]} />;
};

type AccessKind =
  | 'noAccessAndNoTraits'
  | 'noAccessButHasTraits'
  | 'ssoUserAndNoTraits'
  | 'ssoUserButHasTraits';

type ResourceName = 'Kubernetes';

type Props = {
  accessKind: AccessKind;
  resourceName: ResourceName;
  traitDesc: string;
};
