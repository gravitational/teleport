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
import { useAsync } from 'shared/hooks/useAsync';

import { DiscoverUrlLocationState } from 'teleport/Discover/useDiscover';
import {
  IntegrationEnrollEvent,
  IntegrationEnrollEventData,
  IntegrationEnrollKind,
  userEventService,
} from 'teleport/services/userEvent';
import cfg from 'teleport/config';
import {
  Integration,
  IntegrationCreateRequest,
  IntegrationKind,
  integrationService,
  AwsOidcPolicyPreset,
} from 'teleport/services/integrations';

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

  const [createIntegrationAttempt, runCreateIntegration] = useAsync(
    async (req: IntegrationCreateRequest) => {
      const resp = await integrationService.createIntegration(req);
      setCreatedIntegration(resp);
      return resp;
    }
  );

  async function handleOnCreate(validator: Validator) {
    if (!validator.validate()) {
      return;
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
