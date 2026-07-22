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

import styled from 'styled-components';

import { Box, Flex, H3, Text } from 'design';
import { IconTooltip } from 'design/Tooltip';
import TextEditor from 'shared/components/TextEditor';

import useStickyClusterId from 'teleport/useStickyClusterId';

export function ConfigureAwsOidcSummary({
  roleName,
  integrationName,
}: {
  roleName: string;
  integrationName: string;
}) {
  const { clusterId } = useStickyClusterId();

  const json = `{
    "name": ${roleName},
    "description": "Used by Teleport to provide access to AWS resources.",
    "trust_policy": {
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Action": "sts:AssumeRoleWithWebIdentity",
                "Principal": {
                    "Federated": "<YOUR_ACCOUNT_ID>":oidc-provider/${roleName}",
                },
                "Condition": {
                    "StringEquals": {
                        "${clusterId}:aud": "discover.teleport",
                    }
                }
            }
        ]
    },
    "tags": {
        "teleport.dev/cluster": "${clusterId}",
        "teleport.dev/integration": "${integrationName}",
        "teleport.dev/origin": "integration_awsoidc"
    }
}`;

  return (
    <IconTooltip sticky={true} maxWidth={800}>
      <H3 mb={2}>Running the command in AWS CloudShell does the following:</H3>
      <Text>1. Configures an AWS IAM OIDC Identity Provider (IdP)</Text>
      <Text>
        2. Configures an IAM role named "{roleName}" to trust the IdP:
      </Text>
      <Box mb={2}>
        <EditorWrapper>
          <TextEditor
            readOnly={true}
            data={[{ content: json, type: 'json' }]}
            bg="levels.deep"
          />
        </EditorWrapper>
      </Box>
    </IconTooltip>
  );
}

const EditorWrapper = styled(Flex)`
  height: 300px;
  margin-top: ${p => p.theme.space[3]}px;
  width: 700px;
`;
