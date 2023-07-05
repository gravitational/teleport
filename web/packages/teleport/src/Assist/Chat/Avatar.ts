/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import styled from 'styled-components';

export const AvatarContainer = styled.div`
  display: flex;
  align-items: center;
  color: rgba(0, 0, 0, 0.5);

  strong {
    display: block;
    margin-right: 10px;
    color: rgba(0, 0, 0, 0.9);
  }
`;

export const ChatItemAvatarImage = styled.div<{ backgroundImage: string }>`
  background: url(${p => p.backgroundImage}) no-repeat;
  width: 22px;
  height: 22px;
  overflow: hidden;
  background-size: cover;
`;

export const ChatItemAvatarTeleport = styled.div`
  background: #9f85ff;
  padding: 4px;
  border-radius: 10px;
  left: 0;
  right: auto;
  margin-right: 10px;
`;
