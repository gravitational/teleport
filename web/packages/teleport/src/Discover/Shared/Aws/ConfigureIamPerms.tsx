/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useState } from 'react';
import styled from 'styled-components';
import { Flex, Link, Text, ButtonText } from 'design';
import { assertUnreachable } from 'shared/utils/assertUnreachable';
import TextEditor from 'shared/components/TextEditor';

import { CommandBox } from 'teleport/Discover/Shared/CommandBox';
import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import { Regions } from 'teleport/services/integrations';
import cfg from 'teleport/config';

type AwsResourceKind = 'rds' | 'ec2';

export function ConfigureIamPerms({
  region,
  integrationRoleArn,
  kind,
}: {
  region: Regions;
  integrationRoleArn: string;
  kind: AwsResourceKind;
}) {
  const [showRequiredPerms, setShowRequiredPerms] = useState(false);

  // arn's are formatted as `don-care-about-this-part/role-arn`.
  // We are splitting by slash and getting the last element.
  const iamRoleName = integrationRoleArn.split('/').pop();

  let scriptUrl;
  let msg;
  let editor;
  switch (kind) {
    case 'ec2': {
      msg = 'We were unable to list your EC2 instances.';
      scriptUrl = cfg.getEc2InstanceConnectIAMConfigureScriptUrl({
        region,
        iamRoleName,
      });

      const json = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeInstances",
        "ec2:DescribeInstanceConnectEndpoints",
        "ec2:DescribeSecurityGroups",
        "ec2:CreateInstanceConnectEndpoint",
        "ec2:CreateTags",
        "ec2:CreateNetworkInterface",
        "iam:CreateServiceLinkedRole",
        "ec2-instance-connect:SendSSHPublicKey",
        "ec2-instance-connect:OpenTunnel"
      ],
      "Resource": "*"
    }
  ]
}`;

      editor = (
        <EditorWrapper $height={345}>
          <TextEditor
            readOnly={true}
            data={[{ content: json, type: 'json' }]}
          />
        </EditorWrapper>
      );
      break;
    }
    case 'rds': {
      msg = 'We were unable to list your RDS instances.';
      scriptUrl = cfg.getAwsConfigureIamScriptListDatabasesUrl({
        region,
        iamRoleName,
      });

      const json = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "rds:DescribeDBInstances",
        "rds:DescribeDBClusters",
        "ec2:DescribeSecurityGroups"
      ],
      "Resource": "*"
    }
  ]
}`;

      editor = (
        <EditorWrapper $height={235}>
          <TextEditor
            readOnly={true}
            data={[{ content: json, type: 'json' }]}
          />
        </EditorWrapper>
      );
      break;
    }

    default:
      assertUnreachable(kind);
  }

  return (
    <CommandBox
      header={
        <>
          <Text bold>Configure your AWS IAM permissions</Text>
          <Text typography="subtitle1" mb={3}>
            {msg} Run the command below on your{' '}
            <Link
              href="https://console.aws.amazon.com/cloudshell/home"
              target="_blank"
            >
              AWS CloudShell
            </Link>{' '}
            to configure your IAM permissions. Then press the refresh button
            above.
          </Text>
        </>
      }
      hasTtl={false}
    >
      <>
        <TextSelectCopyMulti
          lines={[{ text: `bash -c "$(curl '${scriptUrl}')"` }]}
        />
        <ButtonText mt={3} pl={0} onClick={() => setShowRequiredPerms(b => !b)}>
          View the required IAM perms that will be added as an inline policy to
          IAM role "{iamRoleName}"
        </ButtonText>
        {showRequiredPerms && editor}
      </>
    </CommandBox>
  );
}

const EditorWrapper = styled(Flex)`
  flex-directions: column;
  height: ${p => p.$height}px;
  margin-top: ${p => p.theme.space[3]}px;
`;
