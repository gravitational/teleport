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
import styled from 'styled-components';
import { Text, Box, Indicator } from 'design';
import * as Icons from 'design/Icon';

import {
  Header,
  HeaderSubtitle,
  ActionButtons,
  ButtonBlueText,
} from 'teleport/Discover/Shared';

import { AccessInfo } from './AccessInfo';

import type { TraitKind } from './AccessInfo';
import type { State } from './useUserTraits';

export type Props = {
  isSsoUser: State['isSsoUser'];
  canEditUser: State['canEditUser'];
  attempt: State['attempt'];
  fetchUserTraits: State['fetchUserTraits'];
  headerSubtitle: string;
  traitKind: TraitKind;
  traitDescription: string;
  hasTraits: boolean;
  onProceed(): void;
  onPrev(): void;
  children: React.ReactNode;
  infoContent?: React.ReactNode;
};

export function SetupAccessWrapper({
  attempt,
  fetchUserTraits,
  canEditUser,
  isSsoUser,
  hasTraits,
  traitKind,
  traitDescription,
  headerSubtitle,
  onProceed,
  onPrev,
  children,
  infoContent,
}: Props) {
  const canAddTraits = !isSsoUser && canEditUser;

  let $content;
  switch (attempt.status) {
    case 'failed':
      $content = (
        <>
          <Text my={3}>
            <Icons.Warning ml={1} mr={2} color="danger" />
            Encountered Error: {attempt.statusText}
          </Text>
          <ButtonBlueText ml={1} onClick={fetchUserTraits}>
            Retry
          </ButtonBlueText>
        </>
      );
      break;

    case 'processing':
      $content = (
        <Box mt={4} textAlign="center" height="70px" width="300px">
          <Indicator delay="none" />
        </Box>
      );
      break;

    case 'success':
      if (isSsoUser && !hasTraits) {
        $content = (
          <AccessInfo
            accessKind="ssoUserAndNoTraits"
            traitKind={traitKind}
            traitDesc={traitDescription}
          />
        );
      } else if (!canAddTraits && !hasTraits) {
        $content = (
          <AccessInfo
            accessKind="noAccessAndNoTraits"
            traitKind={traitKind}
            traitDesc={traitDescription}
          />
        );
      } else {
        $content = (
          <>
            <StyledBox>{children}</StyledBox>
            {!isSsoUser && !canEditUser && (
              <AccessInfo
                accessKind="noAccessButHasTraits"
                traitKind={traitKind}
                traitDesc={traitDescription}
              />
            )}
            {isSsoUser && (
              <AccessInfo
                accessKind="ssoUserButHasTraits"
                traitKind={traitKind}
                traitDesc={traitDescription}
              />
            )}
          </>
        );
      }

      break;
  }

  return (
    <Box maxWidth="700px">
      <Header>Set Up Access</Header>
      <HeaderSubtitle>{headerSubtitle}</HeaderSubtitle>
      <Box mb={3}>{$content}</Box>
      {infoContent}
      <ActionButtons
        onProceed={onProceed}
        onPrev={onPrev}
        disableProceed={
          attempt.status === 'failed' ||
          attempt.status === 'processing' ||
          !hasTraits
        }
      />
    </Box>
  );
}

const StyledBox = styled(Box)`
  max-width: 700px;
  background-color: ${props => props.theme.colors.spotBackground[0]};
  border-radius: 8px;
  padding: 20px;
`;
