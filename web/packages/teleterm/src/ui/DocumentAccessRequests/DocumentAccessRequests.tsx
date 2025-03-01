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

import { RequestFlags } from 'shared/components/AccessRequests/ReviewRequests';
import { Attempt } from 'shared/hooks/useAsync';
import { AccessRequest } from 'shared/services/accessRequests';

import Document from 'teleterm/ui/Document';
import * as types from 'teleterm/ui/services/workspacesService';

import { NewRequest } from './NewRequest';
import { RequestList } from './RequestList/RequestList';
import { ReviewAccessRequest } from './ReviewAccessRequest';
import useAccessRequests from './useAccessRequests';
import { useAssumeAccess } from './useAssumeAccess';

export function DocumentAccessRequests(props: DocumentProps) {
  const state = useAccessRequests(props.doc);
  const { assumeRole, assumeRoleAttempt, assumeAccessList } = useAssumeAccess();
  return (
    <Document doc={props.doc} visible={props.visible}>
      <DocumentAccessRequestsViews
        {...state}
        assumeRole={assumeRole}
        assumeRoleAttempt={assumeRoleAttempt}
        assumeAccessList={assumeAccessList}
      />
    </Document>
  );
}

export function DocumentAccessRequestsViews({
  attempt,
  doc,
  assumeRole,
  assumeRoleAttempt,
  getRequests,
  goBack,
  onViewRequest,
  assumeAccessList,
  getFlags,
}: DocumentAccessRequestsProps & {
  assumeRole(requestId: string): void;
  assumeRoleAttempt: Attempt<void>;
  assumeAccessList(): void;
  getFlags(accessRequest: AccessRequest): RequestFlags;
}) {
  if (doc.state === 'creating') {
    return <NewRequest />;
  }

  if (doc.state === 'reviewing') {
    return <ReviewAccessRequest requestId={doc.requestId} goBack={goBack} />;
  }

  return (
    <RequestList
      assumeRole={accessRequest => assumeRole(accessRequest.id)}
      attempt={attempt}
      getFlags={getFlags}
      getRequests={getRequests}
      viewRequest={(id: string) => onViewRequest(id)}
      assumeRoleAttempt={assumeRoleAttempt}
      assumeAccessList={assumeAccessList}
    />
  );
}

export type DocumentAccessRequestsProps = ReturnType<typeof useAccessRequests>;

type DocumentProps = {
  visible: boolean;
  doc: types.DocumentAccessRequests;
};
