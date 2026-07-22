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

import { Alert, Card, Flex, Text } from 'design';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';

import { IntegrationKind } from 'teleport/services/integrations';

type IaCIntegrationKind = IntegrationKind.AwsOidc | IntegrationKind.AzureOidc;

const cloudName: Record<IaCIntegrationKind, string> = {
  [IntegrationKind.AwsOidc]: 'AWS',
  [IntegrationKind.AzureOidc]: 'Azure',
};

type DeleteIntegrationSectionProps = {
  integrationName: string;
  kind: IaCIntegrationKind;
};

export function DeleteIntegrationSection({
  integrationName,
  kind,
}: DeleteIntegrationSectionProps) {
  const cloud = cloudName[kind];

  return (
    <Card>
      <Flex flexDirection="column" p={4} mt={3}>
        <Text fontSize={4} fontWeight="medium" mb={3}>
          Delete Integration
        </Text>

        <Alert kind="danger" mb={3}>
          <Flex flexDirection="column" gap={2}>
            <Text fontWeight="medium">
              Deleting{' '}
              <Text as="strong" fontWeight="bold">
                {integrationName}
              </Text>{' '}
              module from your Terraform configuration will remove Teleport and{' '}
              {cloud} resources used for auto-discovery.
            </Text>
          </Flex>
        </Alert>

        <Text mt={3} mb={2}>
          To delete this integration, remove the module from your Terraform
          configuration and run the command below:
        </Text>

        <TextSelectCopyMulti lines={[{ text: 'terraform apply' }]} />

        <Text mt={2} fontSize={1} color="text.slightlyMuted">
          Note: This removes the integration and dependent resources but does
          not delete your {cloud} resources in Teleport. To remove resources
          from Teleport, delete them via the Teleport UI or CLI.
        </Text>
      </Flex>
    </Card>
  );
}
