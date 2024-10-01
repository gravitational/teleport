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

import React, { useEffect, useState } from 'react';
import { Link as InternalRouteLink } from 'react-router-dom';
import styled from 'styled-components';
import { Box, ButtonSecondary, Text, Link, Flex, ButtonPrimary } from 'design';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';
import { Header } from 'teleport/Discover/Shared';

import { Integration, IntegrationSpecGitHub } from 'teleport/services/integrations';
import cfg from 'teleport/config';
import useGitHub from 'teleport/Integrations/Enroll/GitHub/useGitHub';
import { ConfigureOAuth } from 'teleport/Integrations/Enroll/GitHub/ConfigureOAuth';
import { Resource } from 'teleport/services/resources';
import { CreateIntegration } from 'teleport/Integrations/Enroll/GitHub/CreateIntegration';
import { ConfigureGitHub } from 'teleport/Integrations/Enroll/GitHub/ConfigureGitHub';
import { GitServer } from 'teleport/services/gitservers';
import { CreateGitServer } from 'teleport/Integrations/Enroll/GitHub/CreateGitServer';
import { ConfigureAccess } from 'teleport/Integrations/Enroll/GitHub/ConfigureAccess';
import useTeleport from 'teleport/useTeleport';
import { Usage } from 'teleport/Integrations/Enroll/GitHub/Usage';

export function GitHub() {
  const ctx = useTeleport();
  const cluster = ctx.storeUser.state.cluster;

  // State in sequence.
  const [spec, setSpec] = useState<IntegrationSpecGitHub>();
  const [createdIntegration, setCreatedIntegration] = useState<Integration>();
  const [isCAExported, setCAExported] = useState(false);
  const [gitServer, setGitServer] = useState<GitServer>();
  const [isAccessSet, setAccessSet] = useState(false);

  if (!spec) {
    return (
      <ConfigureOAuth
        cluster={cluster}
        onCreatedSpec={setSpec}
      />
    )
  }
  if (!createdIntegration) {
    return (
      <CreateIntegration
        spec={spec}
        onCreatedIntegration={setCreatedIntegration}
      />
    )
  }
  if (!isCAExported) {
    return (
      <ConfigureGitHub
        integration={createdIntegration}
        onNext={() =>setCAExported(true)}
      />
    )
  }
  if (!gitServer) {
    return(
      <CreateGitServer
        organizationName={createdIntegration.spec.organization}
        integrationName={createdIntegration.name}
        onCreatedGitServer={setGitServer}
      />
    )
  }
  if (!isAccessSet) {
    return(
      <ConfigureAccess
        resourceService={ctx.resourceService}
        organizationName={createdIntegration.spec.organization}
        onNext={()=>setAccessSet(true)}
      />
    )
  }

  return (
    <Usage
      integration={createdIntegration}
      />
  )
}