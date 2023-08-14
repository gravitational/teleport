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

import React, { useState } from 'react';
import styled from 'styled-components';

import { useAssist } from 'teleport/Assist/context/AssistContext';
import { ConversationHistoryItem } from 'teleport/Assist/ConversationHistory/ConversationHistoryItem';
import { DeleteConversationDialog } from 'teleport/Assist/ConversationHistory/DeleteConversationDialog';
import { ViewMode } from 'teleport/Assist/types';

interface ConversationHistoryProps {
  onConversationSelect: (id: string) => void;
  viewMode: ViewMode;
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

function isExpanded(viewMode: ViewMode) {
  return (
    viewMode === ViewMode.PopupExpanded ||
    viewMode === ViewMode.PopupExpandedSidebarVisible
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
