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

import React from 'react';

import { AwsOidcHeader } from 'teleport/Integrations/status/AwsOidc/AwsOidcHeader';
import { useAwsOidcStatus } from 'teleport/Integrations/status/AwsOidc/useAwsOidcStatus';
import { FeatureBox } from 'teleport/components/Layout';

// todo (michellescripts) after routing, ensure this view can be sticky
export function AwsOidcDashboard() {
  const { attempt } = useAwsOidcStatus();

  return (
    <FeatureBox css={{ maxWidth: '1400px', paddingTop: '16px' }}>
      {attempt.data && <AwsOidcHeader integration={attempt.data} />}
      Status for integration type aws-oidc is not supported
    </FeatureBox>
  );
}
