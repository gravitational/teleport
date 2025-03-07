/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { getErrMessage } from 'shared/utils/errorType';

import { App } from 'teleport/services/apps/types';
import {
  CreateAwsAppAccessRequest,
  EnrollEksClustersRequest,
  EnrollEksClustersResponse,
  integrationService,
} from 'teleport/services/integrations';
import TeleportContext from 'teleport/teleportContext';

import { ApiError } from '../api/parseError';
import { JoinToken, JoinTokenRequest } from '../joinToken';

export const ProxyRequiresUpgrade = 'Ensure all proxies are upgraded';

/**
 * Throws a custom error that are a result of `path not found` as a generic
 * error message about this request not being supported and suggests
 * to user to upgrade all proxies to the specified version.
 *
 * Else, rethrows the same error.
 *
 * @param supportedVersion the minimum version required for this
 * request to succeed eg: v17.3.0
 */
export function withGenericUnsupportedError(
  err: unknown,
  supportedVersion: string
) {
  if (err instanceof ApiError && err.response.status === 404) {
    if (err.proxyVersion) {
      throw new Error(
        'We could not complete your request. ' +
          `Your proxy (${err.proxyVersion.string}) may be behind the ` +
          `minimum required version (${supportedVersion}) to support ` +
          `this request. ${ProxyRequiresUpgrade} and try again.`
      );
    }
    // DELETE IN 19.0
    // pre v17 this is the legacy error message crafted as a result
    // of no path found.
    if (err.message == `${err.response.status} - ${err.response.url}`) {
      throw new Error(
        'We could not complete your request. ' +
          'Your proxy may be behind the minimum required version ' +
          `(${supportedVersion}) to support this request. ` +
          `${ProxyRequiresUpgrade} and try again.`
      );
    }
  }
  throw err;
}

export function withUnsupportedLabelFeatureErrorConversion(
  err: unknown
): never {
  if (err instanceof ApiError && err.response.status === 404) {
    throw new Error(
      'We could not complete your request. ' +
        'Your proxy may be behind the minimum required version ' +
        `(v17.2.0) to support adding resource labels. ` +
        `${ProxyRequiresUpgrade} or remove labels and try again.`
    );
  }
  throw err;
}

export const withUnsupportedOktaPluginUpdateErrorConversion = (
  err: unknown
) => {
  if (err instanceof ApiError && err.response.status === 404) {
    const msg = getErrMessage(err);
    throw new Error(
      `Could not update Okta plugin: ${msg}. Your proxy may be behind the minimum required version (v17.3.0) to support Okta plugin updates with this web client.`
    );
  }
  throw err;
};

export const withUnsupportedOktaPluginCreateErrorConversion = (
  err: unknown
) => {
  if (err instanceof ApiError) {
    const msg = getErrMessage(err);
    if (msg.match(/missing okta (?:organization url|api token)/gi)) {
      throw new Error(
        `Could not create Okta plugin: ${msg}. Your proxy may be behind the minimum required version (v17.3.0) to support Okta plugin creation with this web client.`
      );
    }
  }
  throw err;
};

type Base = {
  err: Error;
};

type CreateJoinToken = Base & {
  kind: 'create-join-token';
  req: JoinTokenRequest;
  ctx: TeleportContext;
  abortSignal?: AbortSignal;
};

type EnrollEks = Base & {
  kind: 'enroll-eks';
  req: EnrollEksClustersRequest;
  integrationName: string;
};

type CreateAppAccess = Base & {
  kind: 'create-app-access';
  req: CreateAwsAppAccessRequest;
  integrationName: string;
};

type FallbackProps = CreateJoinToken | EnrollEks | CreateAppAccess;

/**
 * TODO(kimlisa): DELETE IN 19.0
 *
 * Used to fetch with v1 endpoints as a fallback, if its v2 equivalent
 * endpoint failed.
 *
 * Only supports v1 endpoints with equivalent v2 endpoints related to
 * setting resource labels. Related v1 endpoints does not support labels.
 *
 * Fetch is only performed if the v2 error (passed in as a retry prop for
 * function "tryV1Fallback") is a result of requiring a proxy upgrade:
 *  - if api request does not contain any labels,
 *    it will retry with the v1 endpoint without user knowledge
 *  - if api request includes labels, then it will re-throw the error
 *
 * Any other errors will get re-thrown.
 *
 * @returns type FallbackProps
 */
export function useV1Fallback() {
  function hasLabels(props: FallbackProps): number {
    if (props.kind === 'enroll-eks') {
      return props.req.extraLabels.length;
    }
    if (props.kind === 'create-app-access') {
      return props.req.labels && Object.keys(props.req.labels).length;
    }
    if (props.kind === 'create-join-token') {
      return props.req.suggestedLabels.length;
    }
  }

  async function tryV1Fallback(props: CreateAppAccess): Promise<App>;

  async function tryV1Fallback(
    props: EnrollEks
  ): Promise<EnrollEksClustersResponse>;

  async function tryV1Fallback(props: CreateJoinToken): Promise<JoinToken>;

  async function tryV1Fallback(props: FallbackProps) {
    if (!props.err.message.includes(ProxyRequiresUpgrade) || hasLabels(props)) {
      throw props.err;
    }

    if (props.kind === 'enroll-eks') {
      return integrationService.enrollEksClusters(
        props.integrationName,
        props.req
      );
    }

    if (props.kind === 'create-app-access') {
      return integrationService.createAwsAppAccess(props.integrationName);
    }

    if (props.kind === 'create-join-token') {
      return props.ctx.joinTokenService.fetchJoinToken(
        props.req,
        props.abortSignal
      );
    }
  }

  return {
    tryV1Fallback,
  };
}
