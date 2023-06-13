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

import React from 'react';
import styled from 'styled-components';

import { CloseIcon } from 'design/SVGIcon';

import type { Conversation } from 'teleport/Assist/types';

interface ConversationHistoryItemProps {
  conversation: Conversation;
  active: boolean;
  onSelect: () => void;
  onDelete: (id: string) => void;
}

const Buttons = styled.div`
  position: absolute;
  right: 5px;
  top: 0;
  bottom: 0;
  display: none;
  align-items: center;
  justify-content: center;
`;

const Container = styled.li<{ active: boolean }>`
  margin: 0;
  padding: 5px 11px;
  display: flex;
  position: relative;
  justify-content: space-between;
  border-radius: 5px;
  cursor: pointer;
  background: ${p => (p.active ? p.theme.colors.spotBackground[0] : 'none')};

  &:hover {
    background: ${p => p.theme.colors.spotBackground[0]};
    padding-right: 36px;

    ${Buttons} {
      display: flex;
    }
  }

  --title-weight: ${p => (p.active ? 600 : 400)};
`;

const Title = styled.h3`
  margin: 0;
  font-weight: var(--title-weight);
  font-size: 15px;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
`;

const DeleteButton = styled.div`
  cursor: pointer;
  padding: 5px 5px 0;
  background: ${p => p.theme.colors.spotBackground[0]};
  border-radius: 7px;

  &:hover {
    background: ${p => p.theme.colors.error.main};

    svg path {
      stroke: white;
    }
  }
`;

export function ConversationHistoryItem(props: ConversationHistoryItemProps) {
  function handleDelete(event: React.MouseEvent<HTMLDivElement>) {
    event.preventDefault();
    event.stopPropagation();

    props.onDelete(props.conversation.id);
  }

  return (
    <Container active={props.active} onClick={props.onSelect}>
      <Title>{props.conversation.title}</Title>

      <Buttons>
        <DeleteButton onClick={handleDelete}>
          <CloseIcon size={16} />
        </DeleteButton>
      </Buttons>
    </Container>
  );
}
