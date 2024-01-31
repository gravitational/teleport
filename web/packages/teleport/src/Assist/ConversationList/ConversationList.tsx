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

import React, { useEffect, useRef } from 'react';
import styled from 'styled-components';

import { AssistViewMode } from 'gen-proto-ts/teleport/userpreferences/v1/assist_pb';

import { Conversation } from 'teleport/Assist/Conversation';
import { useAssist } from 'teleport/Assist/context/AssistContext';
import { MessageBox } from 'teleport/Assist/MessageBox';

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

    const scrollRefCurrent = scrollRef.current;

    function onscroll() {
      const scrollPosition = scrollRef.current.scrollTop;
      const maxScrollPosition =
        scrollRefCurrent.scrollHeight - scrollRefCurrent.clientHeight;

      // if the user has scrolled more than 50px from the bottom of the chat, assume they don't want the message list
      // to auto scroll.
      shouldScroll.current = scrollPosition > maxScrollPosition - 50;
    }

    scrollRefCurrent.addEventListener('wheel', onscroll);

    return () => scrollRefCurrent.removeEventListener('wheel', onscroll);
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
