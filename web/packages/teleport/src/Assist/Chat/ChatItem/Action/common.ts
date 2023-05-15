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

import { Type } from 'teleport/Assist/services/messages';

export const Container = styled.div`
  border: 1px solid rgba(255, 255, 255, 0.18);
  border-radius: 10px;
  padding: 15px 20px;
  position: relative;
  width: 100%;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
`;

export const Title = styled.div`
  font-size: 14px;
  margin-bottom: 10px;
`;

export const Items = styled.div`
  display: flex;
  flex-wrap: wrap;
  margin-top: -10px;
  align-items: center;

  > * {
    margin-top: 10px;
  }
`;

export function getTextForType(type: Type) {
  switch (type) {
    case Type.ExecuteRemoteCommand:
      return 'Connect to';
    case Type.Message:
      return '';
  }
}
