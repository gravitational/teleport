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

import { useState } from 'react';
import styled from 'styled-components';

import {
  Box,
  ButtonSecondary,
  ButtonText,
  Flex,
  H3,
  Mark,
  Subtitle3,
  Text,
} from 'design';
import * as Icons from 'design/Icon';
import type { Attempt } from 'shared/hooks/useAttemptNext';

import { YamlReader } from 'teleport/Discover/Shared/SetupAccess/AccessInfo';
import type { ConnectionDiagnostic } from 'teleport/services/agents';

import { StyledBox, TextIcon } from '..';

export function ConnectionDiagnosticResult({
  attempt,
  diagnosis,
  canTestConnection,
  testConnection,
  stepNumber,
  stepDescription,
  numberAndDescriptionOnSameLine,
}: Props) {
  const showDiagnosisOutput = !!diagnosis || attempt.status === 'failed';

  let $diagnosisStateComponent;
  if (attempt.status === 'processing') {
    $diagnosisStateComponent = (
      <TextIcon>
        <Icons.Restore size="medium" mr={2} />
        Testing in-progress
      </TextIcon>
    );
  } else if (attempt.status === 'failed' || (diagnosis && !diagnosis.success)) {
    $diagnosisStateComponent = (
      <TextIcon>
        <Icons.Warning size="medium" ml={1} mr={1} color="error.main" />
        Testing failed
      </TextIcon>
    );
  } else if (attempt.status === 'success' && diagnosis?.success) {
    $diagnosisStateComponent = (
      <TextIcon>
        <Icons.CircleCheck size="medium" ml={1} mr={1} color="success.main" />
        Testing complete
      </TextIcon>
    );
  }

  return (
    <StyledBox mb={5}>
      {numberAndDescriptionOnSameLine ? (
        <H3 mb={3}>
          Step {stepNumber}: {stepDescription}
        </H3>
      ) : (
        <header>
          <H3>Step {stepNumber}</H3>
          <Subtitle3 mb={3}>{stepDescription}</Subtitle3>
        </header>
      )}
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
                    <TextIcon key={index}>
                      <Icons.CircleCheck
                        size="medium"
                        color="success.main"
                        mr={1}
                      />
                      {trace.details}
                    </TextIcon>
                  );
                }

                // For whatever reason the status is not the value
                // of failed or success.
                return (
                  <TextIcon key={index}>
                    <Icons.Question size="medium" mr={1} />
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

export const ErrorWithDetails = ({
  details,
  error,
}: {
  details: string;
  error: string;
}) => {
  const [showMore, setShowMore] = useState(false);
  return (
    <TextIcon>
      <Icons.CircleCross size="medium" mr={1} color="error.main" />
      <div>
        <TextWithLineBreaksPreserved>{details}</TextWithLineBreaksPreserved>
        <div>
          <ButtonShowMore onClick={() => setShowMore(p => !p)}>
            {showMore ? 'Hide' : 'Click for extra'} details
          </ButtonShowMore>
          {showMore && (
            <TextWithLineBreaksPreserved>{error}</TextWithLineBreaksPreserved>
          )}
        </div>
      </div>
    </TextIcon>
  );
};

/**
 * Preserves line breaks in traces returned from connection diagnostic.
 */
const TextWithLineBreaksPreserved = styled(Text)`
  white-space: pre-wrap;
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
  numberAndDescriptionOnSameLine?: boolean;
};
