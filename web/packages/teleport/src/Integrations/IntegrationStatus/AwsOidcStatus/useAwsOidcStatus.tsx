/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React, {
  useContext,
  useState,
  useEffect,
  useCallback,
  createContext,
} from 'react';
import { useHistory, useLocation, useParams } from 'react-router';
import { useAsync, Attempt } from 'shared/hooks/useAsync';

import {
  Integration,
  integrationService,
  IntegrationKind,
} from 'teleport/services/integrations';

import useTeleport from 'teleport/useTeleport';

export interface AwsOidcStatusContextState {
  attempt: Attempt<Integration>;
}

const awsOidcStatusContext = createContext<AwsOidcStatusContextState>(null);

export function AwsOidcStatusProvider({
  children,
}: React.PropsWithChildren<unknown>) {
  const { type: integrationType, name: integrationName } = useParams<{
    type: IntegrationKind;
    name: string;
  }>();
  const ctx = useTeleport();

  // // required checks for AWS console and cli access panel
  // // - app fetch, click launch app: do they have the access to fetch a app? does it exist?
  // // - enable button: doesn't exist, requires enrolling aws cli, do they have all the preliminary access to enroll?
  // // - no access disabled state: tooltip no access????
  // const appAccess = ctx.storeUser.getAppServerAccess();
  // const hasAppFetchAccess = appAccess.list && appAccess.read;
  // const hasAppUpsertAccess = appAccess.list && appAccess.read; // to enroll?

  const integrationAccess = ctx.storeUser.getIntegrationsAccess();
  const hasIntegrationReadAccess = integrationAccess.read;

  const [attempt, fetchIntegration] = useAsync(async () => {
    // First check if integration is found before doing other api calls.
    // A user can land on this route, without going through the integration list -> view status flow
    const integration =
      await integrationService.fetchIntegration(integrationName);

    console.log('---- fetched integration: ', integration);

    return integration;
  });

  useEffect(() => {
    // Confirm that integrations exist for any aws oidc related
    // routes (dashboard, tasks, resource listing).
    // User can come from any of those routes (bookmarks).
    // TODO: do a health ping check? just b/c a integration exists, doesn't mean it actually works
    // double check if integration "running" means its actually running???
    if (hasIntegrationReadAccess) {
      fetchIntegration();
    }
  }, []);

  const value: AwsOidcStatusContextState = {
    attempt,
  };

  return (
    <awsOidcStatusContext.Provider value={value}>
      {children}
    </awsOidcStatusContext.Provider>
  );
}

export function useAwsOidcStatus(): AwsOidcStatusContextState {
  return useContext(awsOidcStatusContext);
}
