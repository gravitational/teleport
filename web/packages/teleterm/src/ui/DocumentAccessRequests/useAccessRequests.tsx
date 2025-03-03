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

import { useEffect } from 'react';

import { Timestamp } from 'gen-proto-ts/google/protobuf/timestamp_pb';
import { AccessRequest as TshdAccessRequest } from 'gen-proto-ts/teleport/lib/teleterm/v1/access_request_pb';
import { LoggedInUser } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';
import { RequestFlags } from 'shared/components/AccessRequests/ReviewRequests';
import { mapAttempt } from 'shared/hooks/useAsync';
import {
  AccessRequest,
  makeAccessRequest,
} from 'shared/services/accessRequests';

import { useAccessRequestsContext } from 'teleterm/ui/AccessRequests/AccessRequestsContext';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { useWorkspaceLoggedInUser } from 'teleterm/ui/hooks/useLoggedInUser';
import * as types from 'teleterm/ui/services/workspacesService';

export default function useAccessRequests(doc: types.DocumentAccessRequests) {
  const ctx = useAppContext();
  ctx.clustersService.useState();

  const { rootClusterUri, documentsService } = useWorkspaceContext();
  const { fetchRequestsAttempt, fetchRequests } = useAccessRequestsContext();

  const assumed = ctx.clustersService.getAssumedRequests(rootClusterUri);
  const loggedInUser = useWorkspaceLoggedInUser();

  function goBack() {
    const updatedDoc = documentsService.createAccessRequestDocument({
      clusterUri: rootClusterUri,
      state: 'browsing',
    });
    updatedDoc.uri = doc.uri;
    documentsService.update(doc.uri, updatedDoc);
  }

  function onViewRequest(requestId: string) {
    const updatedDoc = documentsService.createAccessRequestDocument({
      clusterUri: rootClusterUri,
      state: 'reviewing',
      requestId,
    });
    updatedDoc.uri = doc.uri;
    documentsService.update(doc.uri, updatedDoc);
  }

  useEffect(() => {
    // Only fetch when visiting RequestList.
    if (doc.state === 'browsing') {
      void fetchRequests();
    }
  }, [doc.state]);

  return {
    ctx,
    attempt: mapAttempt(fetchRequestsAttempt, requests =>
      requests.map(makeUiAccessRequest)
    ),
    onViewRequest,
    doc,
    getRequests: fetchRequests,
    getFlags: (accessRequest: AccessRequest) =>
      makeFlags(accessRequest, assumed, loggedInUser),
    goBack,
  };
}

export function makeUiAccessRequest(request: TshdAccessRequest) {
  return makeAccessRequest({
    ...request,
    created: Timestamp.toDate(request.created),
    expires: Timestamp.toDate(request.expires),
    maxDuration: request.maxDuration && Timestamp.toDate(request.maxDuration),
    requestTTL: request.requestTtl && Timestamp.toDate(request.requestTtl),
    sessionTTL: request.sessionTtl && Timestamp.toDate(request.sessionTtl),
    assumeStartTime:
      request.assumeStartTime && Timestamp.toDate(request.assumeStartTime),
    roles: request.roles,
    reviews: request.reviews.map(review => ({
      ...review,
      created: Timestamp.toDate(review.created),
      assumeStartTime:
        review.assumeStartTime && Timestamp.toDate(review.assumeStartTime),
    })),
    suggestedReviewers: request.suggestedReviewers,
    thresholdNames: request.thresholdNames,
    resources: request.resources,
  });
}

// transform tsdh Access Request type into the web's Access Request
// to promote code reuse
// TODO(gzdunek): Replace with a function from `DocumentAccessRequests/useReviewAccessRequest`.
export function makeFlags(
  request: AccessRequest,
  assumed: Record<string, TshdAccessRequest>,
  loggedInUser: LoggedInUser
): RequestFlags {
  const ownRequest = request.user === loggedInUser?.name;
  const canAssume = ownRequest && request.state === 'APPROVED';
  const isAssumed = !!assumed[request.id];

  const isPromoted =
    request.state === 'PROMOTED' && !!request.promotedAccessListTitle;

  const reviewed = request.reviews.find(r => r.author === loggedInUser?.name);
  const isPendingState = reviewed
    ? reviewed.state === 'PENDING'
    : request.state === 'PENDING';

  return {
    ...request,
    canAssume,
    isAssumed,
    canReview: !ownRequest && isPendingState,
    canDelete: true,
    ownRequest,
    isPromoted,
  };
}
