import React from 'react';
import { DialogContent, DialogFooter } from 'design/Dialog';
import { Text, Box, ButtonPrimary, Link, Alert, ButtonSecondary } from 'design';

import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { Attempt } from 'shared/hooks/useAttemptNext';

import TextSelectCopy from 'teleport/components/TextSelectCopy';

import { createBashCommand, State } from '../useAddNode';

import type { JoinRule } from 'teleport/services/joinToken';

export default function Iam({ token, attempt, onGenerate, onClose }: Props) {
  const [rule, setRule] = React.useState<JoinRule>({
    awsAccountId: '',
    awsArn: '',
  });

  function handleGenerate(e: React.SyntheticEvent, validator: Validator) {
    e.preventDefault();

    // validate() will run the rule functions of the form inputs
    // it will automatically update the UI with error messages if the validation fails.
    // No need for further actions here in case it fails
    if (!validator.validate()) {
      return;
    }
    onGenerate(rule);
  }

  return (
    <Validation>
      {({ validator }) => (
        <form onSubmit={e => handleGenerate(e, validator)}>
          <DialogContent flex="0 0 auto" minHeight="400px">
            {attempt.status === 'failed' && (
              <Alert kind="danger" children={attempt.statusText} />
            )}
            <Box mb={4}>
              <Text bold as="span" mt={1}>
                Step 1
              </Text>{' '}
              - Assign IAM role to AWS resources
              <Text mt={2}>
                Every node using AWS IAM method to join your Teleport cluster
                needs to be assigned an IAM role.
              </Text>
              <Text mt={1}>
                If it doesn't already exist, create the IAM role "teleport_join"
                and add it to all resources you wish to join your Teleport
                cluster
              </Text>
              <Text mt={1}>
                For more information, see documentation{' '}
                <Link
                  href="https://goteleport.com/docs/setup/guides/joining-nodes-aws/"
                  target="_blank"
                >
                  here
                </Link>
                .
              </Text>
            </Box>
            <Box mb={4}>
              <Text bold as="span" mt={1}>
                Step 2
              </Text>{' '}
              - Specify which nodes can join your Teleport cluster.
              <Box mt={2}>
                <FieldInput
                  label="AWS Account ID"
                  labelTip="nodes must match this AWS Account ID to join your Teleport cluster"
                  autoFocus
                  onChange={e =>
                    setRule({ ...rule, awsAccountId: e.target.value })
                  }
                  rule={requiredAwsAccountId}
                  placeholder="111111111111"
                  value={rule.awsAccountId}
                />
              </Box>
              <FieldInput
                mb={2}
                label="AWS ARN (optional)"
                labelTip="nodes must match this AWS ARN to join your Teleport cluster"
                onChange={e => setRule({ ...rule, awsArn: e.target.value })}
                placeholder="arn:aws:sts::111111111111:assumed-role/teleport-node-role/i-*"
                value={rule.awsArn}
              />
            </Box>
            <Box>
              <Text bold as="span">
                Step 3
              </Text>{' '}
              - Generate and run script
              <ButtonPrimary
                mt={2}
                block
                disabled={attempt.status === 'processing'}
                type="submit"
              >
                Generate Script
              </ButtonPrimary>
              {token && (
                <Box>
                  <Text mt={2}>
                    The token generated is not a secret and will not expire. You
                    can use this script in multiple nodes.
                  </Text>
                  <TextSelectCopy
                    mt="2"
                    text={createBashCommand(token.id, 'iam')}
                  />
                </Box>
              )}
            </Box>
          </DialogContent>
          <DialogFooter>
            <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
          </DialogFooter>
        </form>
      )}
    </Validation>
  );
}

// AWS account ID is a 12 digit string
export const AWS_ACC_ID_REGEXP = /^\d{12}$/;
const requiredAwsAccountId = value => () => {
  const isValidId = value.match(AWS_ACC_ID_REGEXP);
  if (!isValidId) {
    return {
      valid: false,
      message: 'AWS account must be 12 digits',
    };
  }
  return {
    valid: true,
  };
};

type Props = {
  token?: State['iamJoinToken'];
  attempt: Attempt;
  onGenerate(rules: JoinRule): Promise<any>;
  isEnterprise: boolean;
  version: string;
  onClose(): void;
};
