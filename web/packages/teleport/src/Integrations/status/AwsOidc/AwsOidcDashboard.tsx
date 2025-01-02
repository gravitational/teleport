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

import { Flex, H2, Indicator } from 'design';
import { Danger } from 'design/Alert';

import { FeatureBox } from 'teleport/components/Layout';
import { AwsOidcHeader } from 'teleport/Integrations/status/AwsOidc/AwsOidcHeader';
import {
  AwsResource,
  StatCard,
} from 'teleport/Integrations/status/AwsOidc/StatCard';
import { useAwsOidcStatus } from 'teleport/Integrations/status/AwsOidc/useAwsOidcStatus';

export function AwsOidcDashboard() {
  const { statsAttempt, integrationAttempt } = useAwsOidcStatus();

  if (statsAttempt.status == 'processing') {
    return <Indicator />;
  }
  if (statsAttempt.status == 'error') {
    return <Danger>{statsAttempt.statusText}</Danger>;
  }
  if (!statsAttempt.data) {
    return null;
  }

  // todo (michellescripts) after routing, ensure this view can be sticky
  const { awsec2, awseks, awsrds } = statsAttempt.data;
  const { data: integration } = integrationAttempt;
  return (
    <FeatureBox css={{ maxWidth: '1400px', paddingTop: '16px' }}>
      {integration && <AwsOidcHeader integration={integration} />}
      <H2 my={3}>Auto-Enrollment</H2>
      <Flex gap={3}>
        <StatCard resource={AwsResource.ec2} summary={awsec2} />
        <StatCard resource={AwsResource.rds} summary={awsrds} />
        <StatCard resource={AwsResource.eks} summary={awseks} />
      </Flex>
    </FeatureBox>
  );
}
