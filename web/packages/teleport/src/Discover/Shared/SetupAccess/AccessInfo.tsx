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

import {
  kubeAccessRW,
  kubeAccessRO,
  nodeAccessRO,
  nodeAccessRW,
  connDiagRW,
  dbAccessRO,
  dbAccessRW,
} from '../../yamlTemplates';

export function AccessInfo({ accessKind, traitKind, traitDesc }: Props) {
  switch (accessKind) {
    case 'ssoUserAndNoTraits':
      return (
        <>
          <Info>
            You don’t have any {traitKind} {traitDesc} defined.
            <br />
            Please ask your Teleport administrator to update your role and add
            the required {traitKind} {traitDesc}.
          </Info>
          <YamlReader traitKind={traitKind} userAccessReadOnly={true} />
        </>
      );
    case 'noAccessAndNoTraits':
      return (
        <>
          <Info>
            You don’t have {traitKind} access.
            <br />
            Please ask your Teleport administrator to update your role:
          </Info>
          <YamlReader traitKind={traitKind} />
        </>
      );
    case 'noAccessButHasTraits':
      return (
        <>
          <Info>
            You don't have permission to add new {traitKind} {traitDesc}.
            <br />
            If you don't see the {traitKind} {traitDesc} that you require,
            please ask your Teleport administrator to update your role:
          </Info>
          <YamlReader traitKind={traitKind} />
        </>
      );
    case 'ssoUserButHasTraits':
      return (
        <>
          <Info>
            SSO users are not able to add new {traitKind} {traitDesc}.
            <br />
            If you don't see the {traitKind} {traitDesc} that you require,
            please ask your Teleport administrator to update your role:
          </Info>
          <YamlReader traitKind={traitKind} userAccessReadOnly={true} />
        </>
      );
  }
}

export function YamlReader({
  traitKind: resource,
  userAccessReadOnly,
}: {
  traitKind: TraitKind;
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
    case 'OS':
      if (userAccessReadOnly) {
        return (
          <Flex minHeight="150px" mt={3}>
            <ReadOnlyYamlEditor content={nodeAccessRO} />
          </Flex>
        );
      }
      return (
        <Flex minHeight="245px" mt={3}>
          <ReadOnlyYamlEditor content={nodeAccessRW} />
        </Flex>
      );
    case 'Database':
      if (userAccessReadOnly) {
        return (
          <Flex minHeight="210px" mt={3}>
            <ReadOnlyYamlEditor content={dbAccessRO} />
          </Flex>
        );
      }
      return (
        <Flex minHeight="340px" mt={3}>
          <ReadOnlyYamlEditor content={dbAccessRW} />
        </Flex>
      );
    case 'ConnDiag':
      return (
        <Flex minHeight="190px" mt={3}>
          <ReadOnlyYamlEditor content={connDiagRW} />
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
  return (
    <TextEditor
      readOnly={true}
      data={[{ content, type: 'yaml' }]}
      bg="levels.deep"
    />
  );
};

type AccessKind =
  | 'noAccessAndNoTraits'
  | 'noAccessButHasTraits'
  | 'ssoUserAndNoTraits'
  | 'ssoUserButHasTraits';

export type TraitKind = 'Kubernetes' | 'OS' | 'ConnDiag' | 'Database';

type Props = {
  accessKind: AccessKind;
  traitKind: TraitKind;
  traitDesc: string;
};
