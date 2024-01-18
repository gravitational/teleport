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

import { AssistViewMode } from 'gen-proto-ts/teleport/userpreferences/v1/assist_pb';

import { useAssist } from 'teleport/Assist/context/AssistContext';
import { ConversationHistoryItem } from 'teleport/Assist/ConversationHistory/ConversationHistoryItem';
import { DeleteConversationDialog } from 'teleport/Assist/ConversationHistory/DeleteConversationDialog';

interface ConversationHistoryProps {
  onConversationSelect: (id: string) => void;
  viewMode: AssistViewMode;
  onError: (message: string) => void;
}

const Container = styled.ul.attrs({ 'data-scrollbar': 'default' })`
  border-right: 1px solid ${p => p.theme.colors.spotBackground[0]};
  display: flex;
  flex-direction: column;
  gap: 5px;
  box-sizing: border-box;
  list-style: none;
  padding: 0;
  width: var(--conversation-list-width);
  margin: 0;
  position: var(--conversation-list-position);
  top: 0;
  bottom: 0;
  background: ${p => p.theme.colors.levels.popout};
  z-index: 999;
`;

const List = styled.ul.attrs({ 'data-scrollbar': 'default' })`
  display: flex;
  padding: 10px 10px;
  width: 100%;
  flex-direction: column;
  gap: 5px;
  box-sizing: border-box;
  list-style: none;
  overflow-y: auto;
`;

function isExpanded(viewMode: AssistViewMode) {
  return (
    viewMode === AssistViewMode.POPUP_EXPANDED ||
    viewMode === AssistViewMode.POPUP_EXPANDED_SIDEBAR_VISIBLE
  );
}

export function ConversationHistory(props: ConversationHistoryProps) {
  const { conversations, deleteConversation, setSelectedConversationId } =
    useAssist();

  const [deleting, setDeleting] = useState(false);
  const [deleteErrorMessage, setDeleteErrorMessage] = useState<string | null>(
    null
  );
  const [conversationIdToDelete, setConversationIdToDelete] = useState<
    string | null
  >(null);

  async function handleSelectConversation(id: string) {
    try {
      props.onConversationSelect(id);

      await setSelectedConversationId(id);
    } catch (err) {
      props.onError('Failed to load the conversation.');
    }
  }

  async function handleDelete() {
    setDeleteErrorMessage(null);
    setDeleting(true);

    try {
      await deleteConversation(conversationIdToDelete);

      setConversationIdToDelete(null);
    } catch (err) {
      setDeleteErrorMessage(err.message);
    }

    setDeleting(false);
  }

  const conversationToDelete = conversations.data.find(
    conversation => conversation.id === conversationIdToDelete
  );

  const items = conversations.data.map(conversation => (
    <ConversationHistoryItem
      key={conversation.id}
      conversation={conversation}
      active={
        conversations.selectedId === conversation.id &&
        isExpanded(props.viewMode) // avoid showing a background color when the sidebar is a separate view (collapsed & docked)
      }
      onSelect={() => handleSelectConversation(conversation.id)}
      onDelete={() => setConversationIdToDelete(conversation.id)}
    />
  ));

  return (
    <Container>
      {conversationIdToDelete && (
        <DeleteConversationDialog
          conversationTitle={conversationToDelete?.title || ''}
          onDelete={handleDelete}
          onClose={() => setConversationIdToDelete(null)}
          disabled={deleting}
          error={deleteErrorMessage}
        />
      )}

      <List>{items}</List>
    </Container>
  );
}
