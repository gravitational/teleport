/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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
import styled from 'styled-components';
import { Flex, Link, Text, Box } from 'design';
import { assertUnreachable } from 'shared/utils/assertUnreachable';
import TextEditor from 'shared/components/TextEditor';
import { ToolTipInfo } from 'shared/components/ToolTip';

import { CommandBox } from 'teleport/Discover/Shared/CommandBox';
import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import { Regions } from 'teleport/services/integrations';
import cfg from 'teleport/config';

type AwsResourceKind = 'rds' | 'ec2' | 'eks';

export function ConfigureIamPerms({
  region,
  integrationRoleArn,
  kind,
}: {
  region: Regions;
  integrationRoleArn: string;
  kind: AwsResourceKind;
}) {
  // arn's are formatted as `don-care-about-this-part/role-arn`.
  // We are splitting by slash and getting the last element.
  const iamRoleName = integrationRoleArn.split('/').pop();

  let scriptUrl;
  let msg;
  let editor;
  let iamPolicyName;

  switch (kind) {
    case 'ec2': {
      iamPolicyName = 'EC2InstanceConnectEndpoint';
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
            bg="levels.deep"
          />
        </EditorWrapper>
      );
      break;
    }
    case 'eks': {
      iamPolicyName = 'EKSAccess';
      msg = 'We were unable to list your EKS clusters.';
      scriptUrl = cfg.getEksIamConfigureScriptUrl({
        region,
        iamRoleName,
      });

      const json = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "eks:ListClusters",
        "eks:DescribeCluster",
        "eks:ListAccessEntries",
        "eks:CreateAccessEntry",
        "eks:DeleteAccessEntry",
        "eks:AssociateAccessPolicy",
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
            bg="levels.deep"
          />
        </EditorWrapper>
      );
      break;
    }
    case 'rds': {
      iamPolicyName = 'ListDatabases';
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
        <EditorWrapper $height={245}>
          <TextEditor
            readOnly={true}
            data={[{ content: json, type: 'json' }]}
            bg="levels.deep"
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
          <Flex alignItems="center">
            <Text bold mr={1}>
              Configure your AWS IAM permissions
            </Text>
            <ToolTipInfo sticky={true} maxWidth={450}>
              The following IAM permissions will be added as an inline policy
              named <b>{iamPolicyName}</b> to IAM role <b>{iamRoleName}</b>
              <Box mb={2}>{editor}</Box>
            </ToolTipInfo>
          </Flex>
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
      <TextSelectCopyMulti
        lines={[{ text: `bash -c "$(curl '${scriptUrl}')"` }]}
      />
    </CommandBox>
  );
}

const EditorWrapper = styled(Flex)<{ $height: number }>`
  flex-directions: column;
  height: ${p => p.$height}px;
  margin-top: ${p => p.theme.space[3]}px;
  width: 450px;
`;
