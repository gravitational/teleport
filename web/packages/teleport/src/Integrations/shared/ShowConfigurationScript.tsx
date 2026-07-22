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

import { Box, Link, Text } from 'design';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';

export function ShowConfigurationScript({
  scriptUrl,
  description,
}: {
  scriptUrl: string;
  description?: React.ReactNode;
}) {
  return (
    <>
      {description || (
        <Text>
          Open{' '}
          <Link
            href="https://console.aws.amazon.com/cloudshell/home"
            target="_blank"
          >
            AWS CloudShell
          </Link>{' '}
          and copy and paste the command provided below. Upon executing in the
          AWS Shell, the command will download and execute Teleport binary that
          configures Teleport as an OIDC identity provider for AWS and creates
          an IAM role required for the integration.
        </Text>
      )}
      <Box mb={2} mt={3}>
        <TextSelectCopyMulti
          lines={[
            {
              text: `bash -c "$(curl '${scriptUrl}')"`,
            },
          ]}
        />
      </Box>
    </>
  );
}
