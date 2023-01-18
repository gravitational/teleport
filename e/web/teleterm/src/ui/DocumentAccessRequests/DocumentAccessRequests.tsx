import React from 'react';

import * as types from 'teleterm/ui/services/workspacesService';

import Document from 'teleterm/ui/Document';

import useAccessRequests from './useAccessRequests';
import { RequestList } from './RequestList/RequestList';
import { ReviewAccessRequest } from './ReviewAccessRequest';
import { NewRequest } from './NewRequest';

export function DocumentAccessRequests(props: DocumentProps) {
  const state = useAccessRequests(props.doc);
  return (
    <Document doc={props.doc} visible={props.visible}>
      <DocumentAccessRequestsViews {...state} />
    </Document>
  );
}

export function DocumentAccessRequestsViews({
  accessRequests,
  attempt,
  doc,
  assumeRole,
  assumeRoleAttempt,
  getRequests,
  goBack,
  onViewRequest,
}: DocumentAccessRequestsProps) {
  if (doc.state === 'creating') {
    return <NewRequest />;
  }

  if (doc.state === 'reviewing') {
    return <ReviewAccessRequest requestId={doc.requestId} goBack={goBack} />;
  }

  return (
    <RequestList
      assumeRole={assumeRole}
      attempt={attempt}
      requests={accessRequests}
      getRequests={getRequests}
      viewRequest={(id: string) => onViewRequest(id)}
      assumeRoleAttempt={assumeRoleAttempt}
    />
  );
}

export type DocumentAccessRequestsProps = ReturnType<typeof useAccessRequests>;

type DocumentProps = {
  visible: boolean;
  doc: types.DocumentAccessRequests;
};
