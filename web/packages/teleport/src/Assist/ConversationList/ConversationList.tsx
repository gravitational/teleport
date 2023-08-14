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

import React, { useEffect, useLayoutEffect, useRef } from 'react';
import styled from 'styled-components';

import { Conversation } from 'teleport/Assist/Conversation';
import { useAssist } from 'teleport/Assist/context/AssistContext';
import { AssistViewMode } from 'teleport/Assist/Assist';
import { MessageBox } from 'teleport/Assist/MessageBox';
import { LandingPage } from 'teleport/Assist/LandingPage';

interface ConversationListProps {
  viewMode: AssistViewMode;
}

const Container = styled.div.attrs({ 'data-scrollbar': 'default' })`
  flex: 1 1 auto;
  overflow-y: auto;

  &:after {
    content: '';
    display: block;
    height: 10px;
    width: 100%;
  }
`;

export function ConversationList(props: ConversationListProps) {
  const ref = useRef<HTMLDivElement>();

  const scrolling = useRef<boolean>(false);

  const { conversations, selectedConversationMessages } = useAssist();

  function scroll() {
    scrolling.current = true;

    ref.current.scrollIntoView({ behavior: 'smooth' });

    window.setTimeout(() => (scrolling.current = false), 1000);
  }

  useEffect(() => {
    if (!ref.current || scrolling.current) {
      return;
    }

    scroll();
  }, [selectedConversationMessages, scrolling.current]);

  useLayoutEffect(() => {
    if (!ref.current || scrolling.current) {
      return;
    }

    const id = window.setTimeout(scroll, 500);

    return () => window.clearTimeout(id);
  }, [props.viewMode, scrolling.current]);

  if (!conversations.selectedId) {
    return <LandingPage />;
  }

  return (
    <>
      <Container>
        <Conversation />

        <div ref={ref} />
      </Container>
      <MessageBox errorMessage={null} />
    </>
  );
}
