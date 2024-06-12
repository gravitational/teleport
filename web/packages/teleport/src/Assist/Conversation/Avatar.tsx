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

import teleport from 'design/assets/images/icons/teleport.png';

import { useTeleport } from 'teleport';

export const TeleportImage = styled.div<{ backgroundImage: string }>`
  background: url(${teleport}) no-repeat;
  width: 22px;
  height: 22px;
  overflow: hidden;
  background-size: cover;
`;

export const TeleportAvatarContainer = styled.div`
  background: ${props => props.theme.colors.brand};
  padding: 4px;
  border-radius: 10px;
  left: 0;
  right: auto;
  margin-right: 10px;
`;

export function TeleportAvatar() {
  return (
    <TeleportAvatarContainer>
      <TeleportImage />
    </TeleportAvatarContainer>
  );
}

const UserAvatarContainer = styled.div`
  width: 30px;
  height: 30px;
  border-radius: 10px;
  overflow: hidden;
  font-size: 14px;
  font-weight: bold;
  display: flex;
  align-items: center;
  justify-content: center;
  background-size: cover;
  margin-right: 10px;
  background: ${props => props.theme.colors.brand};
  color: ${p => p.theme.colors.buttons.primary.text};
`;

export function UserAvatar() {
  const ctx = useTeleport();

  return (
    <UserAvatarContainer>
      {ctx.storeUser.state.username.slice(0, 1).toUpperCase()}
    </UserAvatarContainer>
  );
}
