/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useState } from 'react';

import { copyToClipboard } from 'design/utils/copyToClipboard';

import { ApiError } from 'teleport/services/api/parseError';
import { integrationService } from 'teleport/services/integrations';
import { userEventService } from 'teleport/services/userEvent';
import {
  IntegrationEnrollCodeType,
  IntegrationEnrollEvent,
  IntegrationEnrollEventData,
  IntegrationEnrollKind,
  IntegrationEnrollStep,
} from 'teleport/services/userEvent/types';

const INTEGRATION_CHECK_RETRIES = 6;
const INTEGRATION_CHECK_RETRY_DELAY = 5000;

type CloudIntegrationKind =
  | IntegrationEnrollKind.AwsCloud
  | IntegrationEnrollKind.AzureCloud;

function generateIntegrationName(kind: CloudIntegrationKind): string {
  const prefix = kind === IntegrationEnrollKind.AwsCloud ? 'aws' : 'azure';

  const randomHex = Array.from(crypto.getRandomValues(new Uint8Array(4)))
    .map(b => b.toString(16).padStart(2, '0'))
    .join('');
  return `${prefix}-integration-${randomHex}`;
}

// useEnrollCloudIntegration returns common steps and state of the IaC Enroll
// flows (AWS, Azure, etc) and handles event tracking for each step
export function useEnrollCloudIntegration(kind: CloudIntegrationKind) {
  const [eventId] = useState(() => crypto.randomUUID());
  const [integrationName, setIntegrationName] = useState(() =>
    generateIntegrationName(kind)
  );

  const integrationQueryKey = ['integration', integrationName];

  function emitEvent(
    event: IntegrationEnrollEvent,
    extra?: Partial<IntegrationEnrollEventData>
  ) {
    userEventService.captureIntegrationEnrollEvent({
      event,
      eventData: {
        id: eventId,
        kind: kind,
        ...extra,
      },
    });
  }

  useEffect(() => {
    emitEvent(IntegrationEnrollEvent.Started);
    // Only send once on init.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const {
    data: integrationData,
    isFetching,
    isError,
    refetch,
  } = useQuery({
    queryKey: integrationQueryKey,
    queryFn: ({ signal }) =>
      integrationService.fetchIntegration(integrationName, signal),
    enabled: false,
    retry: (failureCount, error: unknown) => {
      const shouldRetry =
        failureCount < INTEGRATION_CHECK_RETRIES &&
        error instanceof ApiError &&
        error.response.status === 404;
      return shouldRetry;
    },
    retryDelay: INTEGRATION_CHECK_RETRY_DELAY,
    gcTime: 0,
  });

  const queryClient = useQueryClient();

  const checkIntegration = () => {
    emitEvent(IntegrationEnrollEvent.Step, {
      step: IntegrationEnrollStep.VerifyIntegration,
    });
    refetch().then(result => {
      if (result.isSuccess) {
        emitEvent(IntegrationEnrollEvent.Complete);
      }
    });
  };

  const cancelCheckIntegration = () => {
    queryClient.cancelQueries({ queryKey: integrationQueryKey });
    queryClient.resetQueries({ queryKey: integrationQueryKey });
  };

  const copyTerraformConfig = (config: string) => {
    copyToClipboard(config);
    emitEvent(IntegrationEnrollEvent.CodeCopy, {
      codeType: IntegrationEnrollCodeType.Terraform,
    });
  };

  return {
    integrationName,
    setIntegrationName,
    copyTerraformConfig,
    integrationExists: !!integrationData,
    isFetching,
    isError,
    checkIntegration,
    cancelCheckIntegration,
  };
}
