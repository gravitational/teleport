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

import { Box, Flex, Indicator } from 'design';
import * as Icons from 'design/Icon';
import { P } from 'design/Text/Text';

import {
  ActionButtons,
  ButtonBlueText,
  Header,
  HeaderSubtitle,
} from 'teleport/Discover/Shared';

import { AccessInfo, type TraitKind } from './AccessInfo';
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
  /** A component below the header and above the main content. */
  preContent?: React.ReactNode;
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
  preContent,
  wantAutoDiscover = false,
}: Props) {
  const canAddTraits = !isSsoUser && canEditUser;

  let $content;
  switch (attempt.status) {
    case 'failed':
      // TODO(bl-nero): Migrate this to an alert with embedded retry button.
      $content = (
        <>
          <Flex my={3}>
            <Icons.Warning ml={1} mr={2} color="error.main" size="medium" />
            <P>Encountered Error: {attempt.statusText}</P>
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

  const ssoUserWithAutoDiscover = wantAutoDiscover && isSsoUser;
  return (
    <Box maxWidth="700px">
      <Header>Set Up Access</Header>
      <HeaderSubtitle>{headerSubtitle}</HeaderSubtitle>
      {preContent}
      <Box mb={3}>{$content}</Box>
      {infoContent}
      <ActionButtons
        onProceed={onProceed}
        onPrev={onPrev}
        lastStep={wantAutoDiscover}
        disableProceed={
          attempt.status === 'failed' ||
          attempt.status === 'processing' ||
          // Only block on no traits, if the user is not a SSO user
          // and did not enable auto discover.
          // SSO user's cannot currently add traits and the SSO user
          // may already have set upped traits in their roles, but we
          // currently don't have a way to retrieve all the traits from
          // users roles - in which the user can end up blocked on this step
          // with "no traits".
          (!ssoUserWithAutoDiscover && !hasTraits)
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
