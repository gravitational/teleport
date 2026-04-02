/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { Link as InternalLink } from 'react-router';
import styled from 'styled-components';

import { ButtonPrimary, Flex } from 'design';

import cfg from 'teleport/config';
import { IntegrationKind } from 'teleport/services/integrations';

type CheckIntegrationButtonProps = {
  integrationExists?: boolean;
  integrationName?: string;
  integrationKind: IntegrationKind;
};

export function CheckIntegrationButton({
  integrationExists,
  integrationName,
  integrationKind,
}: CheckIntegrationButtonProps) {
  return (
    <ButtonPrimary
      as={integrationExists && integrationName ? InternalLink : undefined}
      to={
        integrationExists && integrationName
          ? cfg.getIaCIntegrationRoute(integrationKind, integrationName)
          : undefined
      }
      disabled={!integrationExists || !integrationName}
      gap={2}
    >
      View Integration
    </ButtonPrimary>
  );
}
export const Container = styled(Flex)`
  border-radius: 8px;
  background: ${props => props.theme.colors.levels.elevated};

  box-shadow:
    0 2px 1px -1px rgba(0, 0, 0, 0.2),
    0 1px 1px 0 rgba(0, 0, 0, 0.14),
    0 1px 3px 0 rgba(0, 0, 0, 0.12);
`;

export const Divider = styled.hr`
  margin-top: ${p => p.theme.space[3]}px;
  margin-bottom: ${p => p.theme.space[3]}px;
  border: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[0]};
  width: 100%;
`;

export const CircleNumber = styled.span`
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: ${p => p.theme.space[3]}px;
  height: ${p => p.theme.space[3]}px;
  border: 1px solid ${p => p.theme.colors.text.main};
  color: ${p => p.theme.colors.text.main};
  border-radius: 50%;
  font-size: 12px;
  font-weight: 500;
  margin-right: ${p => p.theme.space[2]}px;
  flex-shrink: 0;
  box-sizing: border-box;
`;
