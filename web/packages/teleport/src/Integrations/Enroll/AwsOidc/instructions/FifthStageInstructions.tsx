import React from 'react';

import Text from 'design/Text';
import { ButtonPrimary } from 'design';
import Box from 'design/Box';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';

import { InstructionsContainer } from './common';

import type { CommonInstructionsProps } from './common';

export function FifthStageInstructions(props: CommonInstructionsProps) {
  const policy = `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "rds:DescribeDBInstances",
            "Resource": "*"
        }
    ]
}`;

  return (
    <InstructionsContainer>
      <Text>
        Select the <strong>JSON</strong> tab
      </Text>

      <Text mt={5}>
        Replace the JSON with the following
        <TextSelectCopyMulti lines={[{ text: policy }]} bash={false} />
      </Text>

      <Text mt={5}>
        Click <strong>Next: Tags</strong> and then <strong>Next: Review</strong>
      </Text>

      <Text mt={5}>
        Give the policy a name and then click <strong>Create policy</strong>
      </Text>

      <Box mt={5}>
        <ButtonPrimary onClick={() => props.onNext()}>Next</ButtonPrimary>
      </Box>
    </InstructionsContainer>
  );
}
