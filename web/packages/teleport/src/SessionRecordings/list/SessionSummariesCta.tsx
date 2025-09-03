/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { useCallback, type MouseEvent } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';

import { ChatCircleSparkle, Cross } from 'design/Icon';
import Text from 'design/Text';
import { useLocalStorage } from 'shared/hooks/useLocalStorage';

import { KeysEnum } from 'teleport/services/storageService';

export const CtaLink = styled(Link)`
  border: 1px solid ${p => p.theme.colors.spotBackground[1]};
  border-radius: ${p => p.theme.radii[3]}px;
  padding: ${p =>
    `${p.theme.space[1]}px ${p.theme.space[1]}px ${p.theme.space[1]}px calc(${p.theme.space[2]}px + ${p.theme.space[1]}px)`};
  display: flex;
  gap: ${p => p.theme.space[2]}px;
  align-items: center;
  cursor: pointer;
  color: ${p => p.theme.colors.text.main};
  text-decoration: none;

  &:hover {
    background: ${p => p.theme.colors.spotBackground[0]};
  }
`;

export const DismissButton = styled.button`
  background: transparent;
  border: none;
  border-radius: ${p => p.theme.radii[3]}px;
  padding: ${p => p.theme.space[1]}px;
  margin-left: auto;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0.6;

  &:hover {
    opacity: 1;
    background: ${p => p.theme.colors.spotBackground[0]};
  }
`;

export function SessionSummariesCta() {
  const [dismissed, setDismissed] = useLocalStorage(
    KeysEnum.SESSION_RECORDINGS_DISMISSED_CTA,
    false
  );

  const handleDismiss = useCallback(
    (event: MouseEvent) => {
      event.preventDefault();
      event.stopPropagation();

      setDismissed(true);
    },
    [setDismissed]
  );

  if (dismissed) {
    return null;
  }

  return (
    <CtaLink
      to="https://goteleport.com/docs/identity-security/session-summaries/"
      target="_blank"
    >
      <ChatCircleSparkle size="small" />

      <Text>Summarize session recordings with AI</Text>

      <DismissButton onClick={handleDismiss} aria-label="Dismiss">
        <Cross size="small" />
      </DismissButton>
    </CtaLink>
  );
}
