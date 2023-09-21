/**
 * Copyright 2023 Gravitational, Inc.
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
