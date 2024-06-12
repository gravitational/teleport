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

import React, { useState } from 'react';
import styled from 'styled-components';

import { useAssist } from 'teleport/Assist/context/AssistContext';
import { Message } from 'teleport/Assist/Conversation/Message';
import {
  TypingContainer,
  TypingDot,
} from 'teleport/Assist/Conversation/Typing';
import AuthnDialog from 'teleport/components/AuthnDialog';
import { makeWebauthnAssertionResponse } from 'teleport/services/auth';
import { TeleportAvatar } from 'teleport/Assist/Conversation/Avatar';

const Container = styled.ul`
  list-style: none;
  padding: 0;
  margin: 0;
  flex: 1;
  position: relative;
`;

const Loading = styled.div`
  display: flex;
  align-items: center;
  justify-content: center;
  height: calc(100% - 10px);
  width: inherit;
`;

const Typing = styled.div`
  padding: 0 20px 20px;
  display: flex;
  align-items: center;
  margin-top: -20px;
`;

export function Conversation() {
  const {
    cancelMfaChallenge,
    messages,
    mfa,
    selectedConversationMessages,
    sendMfaChallenge,
  } = useAssist();

  const [mfaErrorMessage, setMfaErrorMessage] = useState<string | null>(null);

  if (messages.loading) {
    return (
      <Loading>
        <TypingContainer>
          <TypingDot style={{ animationDelay: '0s' }} />
          <TypingDot style={{ animationDelay: '0.2s' }} />
          <TypingDot style={{ animationDelay: '0.4s' }} />
        </TypingContainer>
      </Loading>
    );
  }

  if (!selectedConversationMessages) {
    return null;
  }

  const items = selectedConversationMessages.map((message, index) => (
    <Message
      key={index}
      message={message}
      lastMessage={index === selectedConversationMessages.length - 1}
    />
  ));

  async function mfaAuthenticate() {
    if (!window.PublicKeyCredential) {
      const errorText =
        'This browser does not support WebAuthn required for hardware tokens, \
      please try the latest version of Chrome, Firefox or Safari.';

      setMfaErrorMessage(errorText);

      return;
    }

    try {
      const res = await navigator.credentials.get({ publicKey: mfa.publicKey });
      const credential = makeWebauthnAssertionResponse(res);

      sendMfaChallenge(credential);
    } catch (err) {
      setMfaErrorMessage(err.message);
    }
  }

  return (
    <>
      {mfa.prompt && (
        <AuthnDialog
          onContinue={mfaAuthenticate}
          onCancel={cancelMfaChallenge}
          errorText={mfaErrorMessage}
        />
      )}

      <Container>{items}</Container>

      {messages.streaming && (
        <Typing>
          <TeleportAvatar /> <strong>Teleport</strong>
          <TypingContainer>
            <TypingDot style={{ animationDelay: '0s' }} />
            <TypingDot style={{ animationDelay: '0.2s' }} />
            <TypingDot style={{ animationDelay: '0.4s' }} />
          </TypingContainer>
        </Typing>
      )}
    </>
  );
}
