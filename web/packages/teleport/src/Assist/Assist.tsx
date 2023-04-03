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
