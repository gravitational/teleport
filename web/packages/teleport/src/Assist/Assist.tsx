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

import React from 'react';
import styled from 'styled-components';

import { MessagesContextProvider } from 'teleport/Assist/contexts/messages';
import { Sidebar } from 'teleport/Assist/Sidebar';
import { Chat } from 'teleport/Assist/Chat';

const Container = styled.div`
  display: flex;
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  width: 100vw;
  height: 100vh;
  background: #0c143d;
  background-size: cover;
  justify-content: center;
`;

const Width = styled.div`
  display: flex;
  max-width: 1600px;
  width: 100%;
`;

export function Assist() {
  return (
    <MessagesContextProvider>
      <Container>
        <Width>
          <Sidebar />

          <Chat />
        </Width>
      </Container>
    </MessagesContextProvider>
  );
}
