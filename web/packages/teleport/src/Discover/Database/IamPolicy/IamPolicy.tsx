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

import { Box, Flex, H3, Indicator, Link, Text } from 'design';
import * as Icons from 'design/Icon';
import { P } from 'design/Text/Text';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import useTeleport from 'teleport/useTeleport';

import {
  ActionButtons,
  ButtonBlueText,
  Header,
  HeaderSubtitle,
} from '../../Shared';
import type { AgentStepProps } from '../../types';
import { State, useIamPolicy } from './useIamPolicy';

export function IamPolicy(props: AgentStepProps) {
  const ctx = useTeleport();
  const state = useIamPolicy({ ctx, props });

  return <IamPolicyView {...state} />;
}

export function IamPolicyView({
  attempt,
  nextStep,
  iamPolicy,
  fetchIamPolicy,
  iamPolicyName,
}: State) {
  return (
    <Box maxWidth="800px">
      <Header>Configure IAM Policy</Header>
      <HeaderSubtitle>
        Teleport needs AWS IAM permissions to be able to discover and register
        RDS instances and configure IAM authentications.
        <br />
        Optional if you already have an IAM policy configured.
      </HeaderSubtitle>
      {attempt.status === 'failed' ? (
        <>
          <Flex my={3}>
            <Icons.Warning size="medium" ml={1} mr={2} color="error.main" />
            <Text>Encountered Error: {attempt.statusText}</Text>
          </Flex>
          <ButtonBlueText ml={1} onClick={fetchIamPolicy}>
            Retry
          </ButtonBlueText>
        </>
      ) : (
        <Flex height="460px">
          {attempt.status === 'processing' && (
            <Flex width="404px" justifyContent="center" alignItems="center">
              <Indicator />
            </Flex>
          )}
          {attempt.status === 'success' && (
            <Box>
              <H3>Run this AWS CLI command to create an IAM policy:</H3>
              <Box mt={2} mb={2}>
                <TextSelectCopyMulti
                  lines={[
                    {
                      text:
                        `aws iam create-policy \\\n` +
                        `--policy-name ${iamPolicyName} \\\n` +
                        `--policy-document \\\n` +
                        `'${JSON.stringify(
                          JSON.parse(iamPolicy.aws.policy_document),
                          null,
                          2
                        )}'`,
                    },
                  ]}
                />
              </Box>
              <H3>Then attach this policy to your AWS EC2 instance role.</H3>
              <P>
                See{' '}
                <Link
                  href="https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies_manage-attach-detach.html#add-policies-console"
                  target="_blank"
                >
                  Attach policy to an IAM role
                </Link>{' '}
                and{' '}
                <Link
                  href="https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html#attach-iam-role"
                  target="_blank"
                >
                  Attach an IAM role to an instance
                </Link>
              </P>
            </Box>
          )}
        </Flex>
      )}
      <Flex>
        <ActionButtons
          onProceed={nextStep}
          disableProceed={attempt.status !== 'success'}
        />
        <ActionButtons onSkip={() => nextStep(0)} />
      </Flex>
    </Box>
  );
}
