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

import React, { useEffect, useRef } from 'react';
import styled from 'styled-components';

import { Conversation } from 'teleport/Assist/Conversation';
import { useAssist } from 'teleport/Assist/context/AssistContext';
import { MessageBox } from 'teleport/Assist/MessageBox';
import { ViewMode } from 'teleport/Assist/types';

interface ConversationListProps {
  viewMode: ViewMode;
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
  const scrollRef = useRef<HTMLDivElement>();

  const shouldScroll = useRef(true);
  const scrolling = useRef<boolean>(false);
  const setScrollingTimeout = useRef<number>(null);

  const { messages, selectedConversationMessages } = useAssist();

  function scrollIfNotScrolling() {
    if (!shouldScroll.current || scrolling.current) {
      return;
    }

    scrolling.current = true;

    ref.current.scrollIntoView({ behavior: 'smooth' });

    setScrollingTimeout.current = window.setTimeout(() => {
      scrolling.current = false;
    }, 1000);
  }

  useEffect(() => {
    if (!scrollRef.current) {
      return;
    }

    function onscroll() {
      const scrollPosition = scrollRef.current.scrollTop;
      const maxScrollPosition =
        scrollRef.current.scrollHeight - scrollRef.current.clientHeight;

      // if the user has scrolled more than 50px from the bottom of the chat, assume they don't want the message list
      // to auto scroll.
      shouldScroll.current = scrollPosition > maxScrollPosition - 50;
    }

    scrollRef.current.addEventListener('wheel', onscroll);

    return () => scrollRef.current.removeEventListener('wheel', onscroll);
  }, []);

  useEffect(() => {
    if (!ref.current || messages.loading) {
      return;
    }

    scrollIfNotScrolling();
  }, [props.viewMode, messages.loading, selectedConversationMessages]);

  return (
    <>
      <Container ref={scrollRef}>
        <Conversation />

        <div ref={ref} />
      </Container>

      <MessageBox errorMessage={null} />
    </>
  );
}
