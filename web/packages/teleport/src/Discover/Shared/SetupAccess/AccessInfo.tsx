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
            You don’t have any {traitKind} {traitDesc} defined and SSO users are
            not able to add access.
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
