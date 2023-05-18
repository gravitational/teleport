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

import { createPortal } from 'react-dom';

import { useParams } from 'react-router';

import { MessagesContextProvider } from 'teleport/Assist/contexts/messages';
import { ConversationsContextProvider } from 'teleport/Assist/contexts/conversations';
import { ConversationTitle } from 'teleport/Assist/ConversationTitle';
import { LandingPage } from 'teleport/Assist/LandingPage';
import { Chat } from 'teleport/Assist/Chat';
import { Sidebar } from 'teleport/Assist/Sidebar';

const Container = styled.div`
  display: flex;
`;

const ChatContainer = styled.div`
  display: flex;
  max-width: 1600px;
  height: calc(100vh - 72px);
  width: 100%;
`;

export function Assist() {
  const params = useParams<{ conversationId: string }>();

  return (
    <ConversationsContextProvider>
      {params.conversationId ? (
        <MessagesContextProvider
          conversationId={params.conversationId}
          key={params.conversationId}
        >
          <Container>
            <ChatContainer>
              <Chat conversationId={params.conversationId} />
            </ChatContainer>
          </Container>
        </MessagesContextProvider>
      ) : (
        <LandingPage />
      )}

      {createPortal(<Sidebar />, document.getElementById('assist-sidebar'))}
      {createPortal(
        <ConversationTitle />,
        document.getElementById('topbar-portal')
      )}
    </ConversationsContextProvider>
  );
}
