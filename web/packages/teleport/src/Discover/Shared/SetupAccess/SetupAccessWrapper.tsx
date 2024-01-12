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
import styled from 'styled-components';
import { Text, Box, Indicator, Flex } from 'design';
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
  wantAutoDiscover?: boolean;
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
  wantAutoDiscover = false,
}: Props) {
  const canAddTraits = !isSsoUser && canEditUser;

  let $content;
  switch (attempt.status) {
    case 'failed':
      $content = (
        <>
          <Flex my={3}>
            <Icons.Warning ml={1} mr={2} color="error.main" size="medium" />
            <Text>Encountered Error: {attempt.statusText}</Text>
          </Flex>
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
        lastStep={wantAutoDiscover}
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
