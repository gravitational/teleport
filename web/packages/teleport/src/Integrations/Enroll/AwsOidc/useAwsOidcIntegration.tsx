/**
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

import { useEffect, useState } from 'react';
import { useLocation } from 'react-router';

import { Validator } from 'shared/components/Validation';
import {
  makeErrorAttempt,
  makeProcessingAttempt,
  useAsync,
} from 'shared/hooks/useAsync';

import cfg from 'teleport/config';
import { DiscoverUrlLocationState } from 'teleport/Discover/useDiscover';
import { ApiError } from 'teleport/services/api/parseError';
import {
  AwsOidcPolicyPreset,
  Integration,
  IntegrationCreateRequest,
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';
import {
  IntegrationEnrollEvent,
  IntegrationEnrollEventData,
  IntegrationEnrollKind,
  userEventService,
} from 'teleport/services/userEvent';
import useStickyClusterId from 'teleport/useStickyClusterId';

type integrationConfig = {
  name: string;
  roleName: string;
  roleArn: string;
};

export function useAwsOidcIntegration() {
  const [integrationConfig, setIntegrationConfig] = useState<integrationConfig>(
    {
      name: '',
      roleName: '',
      roleArn: '',
    }
  );
  const [scriptUrl, setScriptUrl] = useState('');
  const [createdIntegration, setCreatedIntegration] = useState<Integration>();
  const { clusterId } = useStickyClusterId();

  const location = useLocation<DiscoverUrlLocationState>();

  const [eventData] = useState<IntegrationEnrollEventData>({
    id: crypto.randomUUID(),
    kind: IntegrationEnrollKind.AwsOidc,
  });

  useEffect(() => {
    // If a user came from the discover wizard,
    // discover wizard will send of appropriate events.
    if (location.state?.discover) {
      return;
    }

    emitEvent(IntegrationEnrollEvent.Started);
    // Only send event once on init.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function emitEvent(event: IntegrationEnrollEvent) {
    userEventService.captureIntegrationEnrollEvent({
      event,
      eventData,
    });
  }

  const [
    createIntegrationAttempt,
    runCreateIntegration,
    setCreateIntegrationAttempt,
  ] = useAsync(async (req: IntegrationCreateRequest) => {
    const resp = await integrationService.createIntegration(req);
    setCreatedIntegration(resp);
    return resp;
  });

  async function handleOnCreate(validator: Validator) {
    if (!validator.validate()) {
      return;
    }
    setCreateIntegrationAttempt(makeProcessingAttempt());

    try {
      await integrationService.pingAwsOidcIntegration(
        {
          integrationName: integrationConfig.name,
          clusterId,
        },
        { roleArn: integrationConfig.roleArn }
      );
    } catch (err) {
      // DELETE IN v18.0
      // Ignore not found error and just allow it to create which
      // is how it used to work before anyways.
      //
      // If this request went to an older proxy, that didn't set the
      // the integrationName empty if roleArn isn't empty, then the backend
      // will never be able to successfully health check b/c it expects
      // integration to exist first before creating.
      const isNotFoundErr =
        err instanceof ApiError && err.response.status === 404;

      if (!isNotFoundErr) {
        setCreateIntegrationAttempt(makeErrorAttempt(err));
        return;
      }
    }

    const [, err] = await runCreateIntegration({
      name: integrationConfig.name,
      subKind: IntegrationKind.AwsOidc,
      awsoidc: {
        roleArn: integrationConfig.roleArn,
      },
    });
    if (err) {
      return;
    }

    if (location.state?.discover) {
      return;
    }
    emitEvent(IntegrationEnrollEvent.Complete);
  }

  function generateAwsOidcConfigIdpScript(
    validator: Validator,
    policyPreset: AwsOidcPolicyPreset
  ) {
    if (!validator.validate()) {
      return;
    }

    validator.reset();

    const newScriptUrl = cfg.getAwsOidcConfigureIdpScriptUrl({
      integrationName: integrationConfig.name,
      roleName: integrationConfig.roleName,
      policyPreset,
    });

    setScriptUrl(newScriptUrl);
  }

  return {
    integrationConfig,
    setIntegrationConfig,
    scriptUrl,
    setScriptUrl,
    createdIntegration,
    handleOnCreate,
    runCreateIntegration,
    generateAwsOidcConfigIdpScript,
    createIntegrationAttempt,
  };
}
