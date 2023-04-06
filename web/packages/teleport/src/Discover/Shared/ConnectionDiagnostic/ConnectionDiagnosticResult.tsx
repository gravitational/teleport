/**
 * Copyright 2022 Gravitational, Inc.
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
import { ButtonSecondary, Text, Box, Flex, ButtonText } from 'design';
import * as Icons from 'design/Icon';

import { YamlReader } from 'teleport/Discover/Shared/SetupAccess/AccessInfo';

import { TextIcon, Mark } from '..';

import type { Attempt } from 'shared/hooks/useAttemptNext';
import type { ConnectionDiagnostic } from 'teleport/services/agents';

export function ConnectionDiagnosticResult({
  attempt,
  diagnosis,
  canTestConnection,
  testConnection,
  stepNumber,
  stepDescription,
}: Props) {
  const showDiagnosisOutput = !!diagnosis || attempt.status === 'failed';

  let $diagnosisStateComponent;
  if (attempt.status === 'processing') {
    $diagnosisStateComponent = (
      <TextIcon>
        <Icons.Restore fontSize={4} />
        Testing in-progress
      </TextIcon>
    );
  } else if (attempt.status === 'failed' || (diagnosis && !diagnosis.success)) {
    $diagnosisStateComponent = (
      <TextIcon>
        <Icons.Warning ml={1} color="error.main" />
        Testing failed
      </TextIcon>
    );
  } else if (attempt.status === 'success' && diagnosis?.success) {
    $diagnosisStateComponent = (
      <TextIcon>
        <Icons.CircleCheck ml={1} color="success" />
        Testing complete
      </TextIcon>
    );
  }

  return (
    <StyledBox mb={5}>
      <Text bold>Step {stepNumber}</Text>
      <Text typography="subtitle1" mb={3}>
        {stepDescription}
      </Text>
      <Flex alignItems="center" mt={3}>
        {canTestConnection ? (
          <>
            <ButtonSecondary
              width="200px"
              onClick={testConnection}
              disabled={attempt.status === 'processing'}
            >
              {diagnosis ? 'Restart Test' : 'Test Connection'}
            </ButtonSecondary>
            <Box ml={4}>{$diagnosisStateComponent}</Box>
          </>
        ) : (
          <Box>
            <Text>
              You don't have permission to test connection.
              <br />
              Please ask your Teleport administrator to update your role and add
              the <Mark>connection_diagnostic</Mark> rule:
            </Text>
            <YamlReader traitKind="ConnDiag" />
          </Box>
        )}
      </Flex>
      {showDiagnosisOutput && (
        <Box mt={3}>
          {attempt.status === 'failed' &&
            `Encountered Error: ${attempt.statusText}`}
          {attempt.status === 'success' && (
            <Box>
              {diagnosis.traces.map((trace, index) => {
                if (trace.status === 'failed') {
                  return (
                    <ErrorWithDetails
                      error={trace.error}
                      details={trace.details}
                      key={index}
                    />
                  );
                }
                if (trace.status === 'success') {
                  return (
                    <TextIcon key={index} css={{ alignItems: 'baseline' }}>
                      <Icons.CircleCheck mr={1} color="success" />
                      {trace.details}
                    </TextIcon>
                  );
                }

                // For whatever reason the status is not the value
                // of failed or success.
                return (
                  <TextIcon key={index}>
                    <Icons.Question mr={1} />
                    {trace.details}
                  </TextIcon>
                );
              })}
            </Box>
          )}
        </Box>
      )}
    </StyledBox>
  );
}

const ErrorWithDetails = ({
  details,
  error,
}: {
  details: string;
  error: string;
}) => {
  const [showMore, setShowMore] = useState(false);
  return (
    <TextIcon css={{ alignItems: 'baseline' }}>
      <Icons.CircleCross mr={1} color="error.main" />
      <div>
        <div>{details}</div>
        <div>
          <ButtonShowMore onClick={() => setShowMore(p => !p)}>
            {showMore ? 'Hide' : 'Click for extra'} details
          </ButtonShowMore>
          {showMore && <div>{error}</div>}
        </div>
      </div>
    </TextIcon>
  );
};

const StyledBox = styled(Box)`
  max-width: 800px;
  background-color: ${props => props.theme.colors.spotBackground[0]};
  border-radius: 8px;
  padding: 20px;
`;

const ButtonShowMore = styled(ButtonText)`
  min-height: auto;
  padding: 0;
  font-weight: inherit;
  text-decoration: underline;
`;

export type Props = {
  attempt: Attempt;
  diagnosis: ConnectionDiagnostic;
  canTestConnection: boolean;
  testConnection(): void;
  stepNumber: number;
  stepDescription: string;
};
