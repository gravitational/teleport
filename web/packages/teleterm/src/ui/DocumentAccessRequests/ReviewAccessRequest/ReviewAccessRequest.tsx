/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import styled from 'styled-components';

import { Text, Flex, Box, Alert } from 'design';
import { ArrowBack } from 'design/Icon';
import { makeEmptyAttempt } from 'shared/hooks/useAsync';

import {
  RequestDelete,
  RequestView,
} from 'shared/components/AccessRequests/ReviewRequests';

import { useAssumeAccess } from '../useAssumeAccess';

import { useReviewAccessRequest } from './useReviewAccessRequest';

export function ReviewAccessRequest(props: {
  requestId: string;
  goBack(): void;
}) {
  const {
    fetchRequestAttempt,
    submitReviewAttempt,
    submitReview,
    deleteDialogOpen,
    setDeleteDialogOpen,
    deleteRequest,
    deleteRequestAttempt,
    user,
    getFlags,
    fetchSuggestedAccessListsAttempt,
  } = useReviewAccessRequest(props);
  const { assumeRole, assumeRoleAttempt, assumeAccessList } = useAssumeAccess();

  function getDialogDelete() {
    const hasRequest =
      fetchRequestAttempt.status === 'success' ||
      submitReviewAttempt.status === 'success';
    if (!(deleteDialogOpen && hasRequest)) {
      return;
    }

    const request =
      submitReviewAttempt.status === 'success'
        ? submitReviewAttempt.data
        : fetchRequestAttempt.data;

    return (
      <RequestDelete
        deleteRequestAttempt={deleteRequestAttempt}
        user={request.user}
        roles={request.roles}
        requestId={request.id}
        requestState={request.state}
        onClose={() => setDeleteDialogOpen(false)}
        onDelete={deleteRequest}
      />
    );
  }

  return (
    <Layout mx="auto" px={5} pt={3} height="100%">
      <Header>
        <HeaderTitle typography="h3" mb={3}>
          <Flex alignItems="center">
            <ArrowBack
              mr={2}
              size="large"
              onClick={props.goBack}
              style={{ textDecoration: 'none', cursor: 'pointer' }}
            />
            <Text>{`Request: ${props.requestId}`}</Text>
          </Flex>
        </HeaderTitle>
      </Header>
      {assumeRoleAttempt.status === 'error' && (
        <Alert kind="danger" children={assumeRoleAttempt.statusText} />
      )}
      <RequestView
        user={user?.name}
        fetchRequestAttempt={fetchRequestAttempt}
        getFlags={getFlags}
        confirmDelete={false} // never show the embedded request delete
        toggleConfirmDelete={() => setDeleteDialogOpen(true)}
        submitReview={submitReview}
        assumeRole={() => assumeRole(props.requestId)}
        assumeRoleAttempt={assumeRoleAttempt}
        submitReviewAttempt={submitReviewAttempt}
        fetchSuggestedAccessListsAttempt={fetchSuggestedAccessListsAttempt}
        assumeAccessList={assumeAccessList}
        //TODO(gzdunek): Remove our custom dialog and instead fill the props here.
        deleteRequestAttempt={makeEmptyAttempt()}
        deleteRequest={() => undefined}
      />
      {getDialogDelete()}
    </Layout>
  );
}

const Header = styled(Flex)`
  flex-shrink: 0;
  border-bottom: 1px solid ${props => props.theme.colors.spotBackground[0]};
  height: 56px;
  margin-bottom: 24px;
`;

const HeaderTitle = styled(Text)`
  white-space: nowrap;
`;

const Layout = styled(Box)`
  flex-direction: column;
  display: flex;
  flex: 1;
  max-width: 1248px;
`;
