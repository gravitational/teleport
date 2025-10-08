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

import styled from 'styled-components';

import { Box, ButtonSecondary, Flex, Label } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import * as Icons from 'design/Icon';

interface UserRolesProps {
  isOpen: boolean;
  onClose: () => void;
  roles: string[];
  userName: string;
}

export function UserRoles({ isOpen, onClose, roles, userName }: UserRolesProps) {
  return (
    <Dialog disableEscapeKeyDown={false} onClose={onClose} open={isOpen}>
      <DialogHeader>
        <DialogTitle>All {userName} roles</DialogTitle>
      </DialogHeader>
      
      <DialogContent>
        <Box width="600px" maxHeight="60vh" overflow="auto">
          <SectionTitle>Assigned Roles</SectionTitle>
          <RolesGrid>
            {roles.map(role => (
              <Label key={role} kind="secondary">
                <Flex alignItems="center" gap={1}>
                  <Icons.UserIdBadge size={12} />
                  {role}
                </Flex>
              </Label>
            ))}
          </RolesGrid>
        </Box>
      </DialogContent>
      
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>
          Close
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

const SectionTitle = styled.h4`
  font-size: ${props => props.theme.fontSizes[3]}px;
  font-weight: bold;
  margin: 0 0 ${props => props.theme.space[3]}px 0;
  color: ${props => props.theme.colors.text.main};
`;

const RolesGrid = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: ${props => props.theme.space[2]}px;
`;