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
