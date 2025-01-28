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

import React, { createContext, useContext, useEffect } from 'react';
import { useParams } from 'react-router';

import { Attempt, useAsync } from 'shared/hooks/useAsync';

import {
  IntegrationAwsOidc,
  IntegrationKind,
  integrationService,
  IntegrationWithSummary,
} from 'teleport/services/integrations';
import useTeleport from 'teleport/useTeleport';

export interface AwsOidcStatusContextState {
  statsAttempt: Attempt<IntegrationWithSummary>;
  integrationAttempt: Attempt<IntegrationAwsOidc>;
}

export const awsOidcStatusContext =
  createContext<AwsOidcStatusContextState>(null);

export function AwsOidcStatusProvider({ children }: React.PropsWithChildren) {
  const { name } = useParams<{
    type: IntegrationKind;
    name: string;
  }>();
  const ctx = useTeleport();
  const integrationAccess = ctx.storeUser.getIntegrationsAccess();
  const hasIntegrationReadAccess = integrationAccess.read;

  const [stats, fetchIntegrationStats] = useAsync(() =>
    integrationService.fetchIntegrationStats(name)
  );

  const [integration, fetchIntegration] = useAsync(() =>
    integrationService.fetchIntegration(name)
  );

  useEffect(() => {
    if (hasIntegrationReadAccess) {
      fetchIntegrationStats();
      fetchIntegration();
    }
  }, []);

  const value: AwsOidcStatusContextState = {
    statsAttempt: stats,
    integrationAttempt: integration as Attempt<IntegrationAwsOidc>,
  };

  return (
    <awsOidcStatusContext.Provider value={value}>
      {children}
    </awsOidcStatusContext.Provider>
  );
}

export function useAwsOidcStatus(): AwsOidcStatusContextState {
  const context = useContext(awsOidcStatusContext);
  if (!context) {
    throw new Error(
      'useAwsOidcStatus must be used within a AwsOidcStatusProvider'
    );
  }
  return context;
}
