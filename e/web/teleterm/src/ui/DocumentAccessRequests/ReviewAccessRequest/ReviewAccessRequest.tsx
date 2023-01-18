import React, { useEffect } from 'react';

import styled from 'styled-components';

import { Text, Flex, Box, Alert } from 'design';
import { ArrowBack } from 'design/Icon';

import { RequestDelete } from 'e-teleport/Workflow/ReviewRequests/RequestView/RequestDelete/RequestDelete';
import { RequestView } from 'e-teleport/Workflow/ReviewRequests/RequestView/RequestView';

import useReviewAccessRequest from './useReviewAccessRequest';

export function ReviewAccessRequest(props: Props) {
  const {
    goBack,
    request,
    attempt,
    requestId,
    assumeRole,
    submitReviewAttempt,
    submitReview,
    deleteDialogOpen,
    assumeRoleAttempt,
    setDeleteDialogOpen,
    deleteRequest,
    deleteRequestAttempt,
    user,
    flags,
  } = useReviewAccessRequest(props);
  useEffect(() => {
    if (deleteRequestAttempt.status === 'success') {
      goBack();
    }
  }, [deleteRequestAttempt]);

  return (
    <Layout mx="auto" px={5} pt={3} height="100%">
      <Header>
        <HeaderTitle typography="h3" mb={3}>
          <Flex alignItems="center">
            <ArrowBack
              mr={2}
              fontSize={8}
              onClick={goBack}
              style={{ textDecoration: 'none', cursor: 'pointer' }}
            />
            <Text>{`Request: ${requestId}`}</Text>
          </Flex>
        </HeaderTitle>
      </Header>
      {assumeRoleAttempt.status === 'failed' && (
        <Alert kind="danger" children={assumeRoleAttempt.statusText} />
      )}
      <RequestView
        user={user?.name}
        attempt={attempt}
        request={request}
        flags={flags}
        confirmDelete={false} // never show the embedded request delete
        toggleConfirmDelete={() => setDeleteDialogOpen(true)}
        submitReview={submitReview}
        assumeRole={assumeRole}
        reviewAttempt={submitReviewAttempt}
      />
      {request && deleteDialogOpen && (
        <RequestDelete
          attempt={deleteRequestAttempt}
          user={request?.user}
          roles={request?.roles}
          requestId={request?.id}
          requestState={request?.state}
          onClose={() => setDeleteDialogOpen(false)}
          onDelete={() => deleteRequest(request.id)}
        />
      )}
    </Layout>
  );
}

type Props = {
  requestId: string;
  goBack: () => void;
};

const Header = styled(Flex)`
  flex-shrink: 0;
  border-bottom: 1px solid ${props => props.theme.colors.primary.main};
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
