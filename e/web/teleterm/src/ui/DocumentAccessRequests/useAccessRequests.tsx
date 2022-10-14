import { useState, useEffect } from 'react';

import * as types from 'teleterm/ui/services/workspacesService';
import useAttempt from 'shared/hooks/useAttemptNext';
import { AccessRequest as TshdAccessRequest } from 'teleterm/services/tshd/types';
import { makeAccessRequest, AccessRequest } from 'e-teleport/services/workflow';

import { useIdentity } from 'teleterm/ui/TopBar/Identity/useIdentity';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { retryWithRelogin } from 'teleterm/ui/utils';
import { useWorkspaceContext } from 'teleterm/ui/Documents';

export default function useAccessRequests(doc: types.DocumentAccessRequests) {
  const ctx = useAppContext();
  ctx.clustersService.useState();

  const {
    localClusterUri: clusterUri,
    rootClusterUri,
    documentsService,
  } = useWorkspaceContext();

  const assumed = ctx.clustersService.getAssumedRequests(rootClusterUri);
  const identity = useIdentity();
  const [accessRequests, setAccessRequests] = useState<AccessRequest[]>();
  const { attempt, setAttempt } = useAttempt('');
  const { attempt: assumeRoleAttempt, run: runAssumeRole } = useAttempt('');

  // transform tsdh Access Request type into the web's Access Request
  // to promote code reuse
  function makeRow(request: AccessRequest) {
    const ownRequest =
      request.user === identity?.activeRootCluster?.loggedInUser.name;
    const canAssume = ownRequest && request.state === 'APPROVED';
    const isAssumed = assumed[request.id];

    return {
      ...request,
      canAssume,
      isAssumed,
    };
  }

  function goBack() {
    documentsService.update(doc.uri, {
      title: `Access Requests`,
      state: 'browsing',
      requestId: '',
    });
  }

  function onViewRequest(requestId: string) {
    documentsService.update(doc.uri, {
      title: `Request: ${requestId}`,
      state: 'reviewing',
      requestId,
    });
  }

  const getRequests = async () => {
    try {
      const response = await retryWithRelogin(ctx, doc.uri, clusterUri, () =>
        ctx.clustersService.getAccessRequests(rootClusterUri)
      );
      setAttempt({ status: 'success' });
      // transform tshd access request to the webui access request and add flags
      const requests = response.map(r => makeRow(makeUiAccessRequest(r)));
      setAccessRequests(requests);
    } catch (err) {
      setAttempt({
        status: 'failed',
        statusText: err.message,
      });
    }
  };

  async function assumeRole(request: AccessRequest) {
    runAssumeRole(() =>
      retryWithRelogin(ctx, doc.uri, clusterUri, () =>
        // pass the requestId to the requestIds array on its own, and nothing into the dropids array
        // since we are only 'assuming' one requestId at a time
        ctx.clustersService
          .assumeRole(rootClusterUri, [request.id], [])
          .then(() => {
            ctx.clustersService.syncCluster(clusterUri);
          })
      )
    );
  }

  useEffect(() => {
    // only fetch when visitng RequestList
    if (doc.state === 'browsing') {
      getRequests();
    }
  }, [doc.state, clusterUri]);

  useEffect(() => {
    // if assumed object changes, we update which roles have been assumed in the table
    // this is mostly for using "Switchback" since that state is held outside this component
    setAccessRequests(prevState =>
      prevState?.map(r => ({
        ...r,
        isAssumed: assumed[r.id],
      }))
    );
  }, [assumed]);

  return {
    ctx,
    attempt,
    accessRequests,
    assumeRoleAttempt,
    assumeRole,
    onViewRequest,
    doc,
    getRequests,
    goBack,
  };
}

export function makeUiAccessRequest(request: TshdAccessRequest) {
  return makeAccessRequest({
    ...request,
    // Timestamppb sends date through gRPC
    // {
    //   seconds: number,
    //   nanos: number,
    // },
    created: request.created.seconds * 1000,
    expires: request.expires.seconds * 1000,
    roles: request.rolesList,
    reviews: request.reviewsList.map(review => ({
      ...review,
      created: review.created.seconds * 1000,
    })),
    suggestedReviewers: request.suggestedReviewersList,
    thresholdNames: request.thresholdNamesList,
    // TODO (avatus): fetch detailed data to include hostname after implementing
    // changes for Connect related to https://github.com/gravitational/webapps.e/pull/391
    // for now, use legacy display of node id
    resources: request.resourceIdsList.map(rid => ({
      id: rid,
      details: {},
    })),
  });
}
