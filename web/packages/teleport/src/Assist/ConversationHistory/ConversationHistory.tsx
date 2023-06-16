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

import { ButtonPrimary } from 'design';

import { useAssist } from 'teleport/Assist/context/AssistContext';
import { ConversationHistoryItem } from 'teleport/Assist/ConversationHistory/ConversationHistoryItem';
import { AssistViewMode } from 'teleport/Assist/Assist';
import { DeleteConversationDialog } from 'teleport/Assist/ConversationHistory/DeleteConversationDialog';

interface ConversationHistoryProps {
  onConversationSelect: (id: string) => void;
  viewMode: AssistViewMode;
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
  top: var(--assist-header-height);
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

const NewConversationButton = styled.li`
  margin: 10px 10px 0;

  button {
    width: 100%;
  }
`;

const ErrorMessage = styled.div`
  background: ${props => props.theme.colors.error.main};
  color: white;
  border-radius: 5px;
  margin-bottom: 10px;
  padding: 5px 10px;
`;

function isExpanded(viewMode: AssistViewMode) {
  return (
    viewMode === AssistViewMode.Expanded ||
    viewMode === AssistViewMode.ExpandedSidebarVisible
  );
}

export function ConversationHistory(props: ConversationHistoryProps) {
  const {
    conversations,
    createConversation,
    deleteConversation,
    setSelectedConversationId,
  } = useAssist();

  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteErrorMessage, setDeleteErrorMessage] =
    useState<string | null>(null);
  const [conversationIdToDelete, setConversationIdToDelete] =
    useState<string | null>(null);

  async function handleSelectConversation(id: string) {
    try {
      props.onConversationSelect(id);

      await setSelectedConversationId(id);
    } catch (err) {
      setErrorMessage(err.message);
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

  async function handleCreateNewConversation() {
    try {
      const id = await createConversation();

      props.onConversationSelect(id);
    } catch (err) {
      setErrorMessage(err.message);
    }
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

      <NewConversationButton>
        {errorMessage && <ErrorMessage>{errorMessage}</ErrorMessage>}

        <ButtonPrimary onClick={() => handleCreateNewConversation()}>
          New conversation
        </ButtonPrimary>
      </NewConversationButton>

      <List>{items}</List>
    </Container>
  );
}
