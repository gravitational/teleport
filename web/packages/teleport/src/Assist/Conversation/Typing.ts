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

import styled, { keyframes } from 'styled-components';

const loading = keyframes`
  0% {
    opacity: 0;
  }
  50% {
    opacity: 1;
  }
  100% {
    opacity: 0;
  }
`;

export const Typing = styled.div`
  margin: 0 30px 0 30px;
`;

export const TypingContainer = styled.div`
  position: relative;
  padding: 10px;
  display: flex;
`;

export const TypingDot = styled.div`
  width: 6px;
  height: 6px;
  margin-right: 6px;
  background: ${p => p.theme.colors.spotBackground[2]};
  border-radius: 50%;
  opacity: 0;
  animation: ${loading} 1.5s ease-in-out infinite;
`;
