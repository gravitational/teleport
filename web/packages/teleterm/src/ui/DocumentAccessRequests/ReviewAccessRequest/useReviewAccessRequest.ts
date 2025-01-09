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

import { useCallback, useEffect, useState } from 'react';

import { Timestamp } from 'gen-proto-ts/google/protobuf/timestamp_pb';
import {
  RequestFlags,
  SubmitReview,
  SuggestedAccessList,
} from 'shared/components/AccessRequests/ReviewRequests';
import { useAsync } from 'shared/hooks/useAsync';
import { AccessRequest } from 'shared/services/accessRequests';

import * as tsh from 'teleterm/services/tshd/types';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { useWorkspaceContext } from 'teleterm/ui/Documents';
import { useWorkspaceLoggedInUser } from 'teleterm/ui/hooks/useLoggedInUser';
import { retryWithRelogin } from 'teleterm/ui/utils';

import { makeUiAccessRequest } from '../useAccessRequests';

export function useReviewAccessRequest({
  requestId,
  goBack,
}: {
  requestId: string;
  goBack(): void;
}) {
  const ctx = useAppContext();
  ctx.clustersService.useState();

  const { localClusterUri: clusterUri, rootClusterUri } = useWorkspaceContext();
  const loggedInUser = useWorkspaceLoggedInUser();
  const assumed = ctx.clustersService.getAssumedRequests(rootClusterUri);

  const retry = useCallback(
    <T>(action: () => Promise<T>) => retryWithRelogin(ctx, clusterUri, action),
    [clusterUri, ctx]
  );

  const [fetchRequestAttempt, runFetchRequest] = useAsync(
    useCallback(
      () =>
        retry(async () => {
          const request = await ctx.clustersService.getAccessRequest(
            rootClusterUri,
            requestId
          );
          return makeUiAccessRequest(request);
        }),
      [ctx.clustersService, requestId, retry, rootClusterUri]
    )
  );
  const [deleteRequestAttempt, runDeleteRequest] = useAsync(() =>
    retry(async () => {
      await ctx.tshd.deleteAccessRequest({
        rootClusterUri,
        accessRequestId: requestId,
      });
    })
  );
  const [submitReviewAttempt, runSubmitReview] = useAsync(
    (review: SubmitReview) =>
      retry(async () => {
        // This should not happen because the UI is hidden when fetching the request is in progress.
        if (fetchRequestAttempt.status !== 'success') {
          throw new Error('No access request to review.');
        }

        const updatedAccessRequest =
          review.state === 'PROMOTED' && review.promotedToAccessList
            ? await ctx.clustersService.promoteAccessRequest({
                rootClusterUri,
                accessRequestId: requestId,
                reason: review.reason,
                accessListId: review.promotedToAccessList.id,
              })
            : await ctx.clustersService.reviewAccessRequest({
                rootClusterUri,
                state: review.state,
                reason: review.reason,
                roles: fetchRequestAttempt.data.roles,
                accessRequestId: requestId,
                assumeStartTime:
                  review.assumeStartTime &&
                  Timestamp.fromDate(review.assumeStartTime),
              });

        return makeUiAccessRequest(updatedAccessRequest);
      })
  );
  const [fetchSuggestedAccessListsAttempt, runFetchSuggestedAccessLists] =
    useAsync(
      useCallback(async () => {
        const { response } = await ctx.tshd.getSuggestedAccessLists({
          rootClusterUri,
          accessRequestId: requestId,
        });

        return response.accessLists.map(makeUiAccessList);
      }, [ctx.tshd, requestId, rootClusterUri])
    );

  function getFlags(request: AccessRequest): RequestFlags {
    if (loggedInUser) {
      return getRequestFlags(request, loggedInUser, assumed);
    }
    return undefined;
  }

  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  useEffect(() => {
    if (fetchRequestAttempt.status === '') {
      runFetchRequest();
    }

    if (fetchSuggestedAccessListsAttempt.status === '') {
      runFetchSuggestedAccessLists();
    }
  }, [
    fetchRequestAttempt.status,
    fetchSuggestedAccessListsAttempt.status,
    runFetchRequest,
    runFetchSuggestedAccessLists,
  ]);

  async function deleteRequest(): Promise<void> {
    const [, error] = await runDeleteRequest();
    if (!error) {
      goBack();
    }
  }

  return {
    user: loggedInUser,
    getFlags,
    fetchRequestAttempt,
    submitReviewAttempt,
    deleteDialogOpen,
    setDeleteDialogOpen,
    deleteRequestAttempt,
    deleteRequest,
    submitReview: runSubmitReview,
    fetchSuggestedAccessListsAttempt,
  };
}

function getRequestFlags(
  request: AccessRequest,
  user: tsh.LoggedInUser,
  assumedMap: Record<string, tsh.AssumedRequest>
): RequestFlags {
  const ownRequest = request.user === user.name;
  const canAssume = ownRequest && request.state === 'APPROVED';
  const isAssumed = !!assumedMap[request.id];
  const canDelete = true;

  const reviewed = request.reviews.find(r => r.author === user.name);

  const isPromoted = request.state === 'PROMOTED';

  const isPendingState = reviewed
    ? reviewed.state === 'PENDING'
    : request.state === 'PENDING';

  const canReview = !ownRequest && isPendingState && user.acl.reviewRequests;

  return {
    canAssume,
    isAssumed,
    canDelete,
    canReview,
    isPromoted,
    ownRequest,
  };
}

// Should be kept in sync with accessmanagement.makeAccessList().
function makeUiAccessList(al: tsh.AccessList): SuggestedAccessList {
  const spec = al.spec;
  const metadata = al.header.metadata;

  return {
    id: metadata.name,
    title: spec.title,
    description: spec.description,
    grants: {
      roles: spec.grants.roles.sort(),
      traits: spec.grants.traits.reduce<Record<string, string[]>>(
        (accumulator, trait) => {
          accumulator[trait.key] = trait.values;
          return accumulator;
        },
        {}
      ),
    },
  };
}
