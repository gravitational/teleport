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

import styled from 'styled-components';

import { Text } from 'design';

import TextSelectCopy from 'teleport/components/TextSelectCopy';

interface StepProps {
  stepNumber?: number;
  title: string;
  text: string;
  isBash?: boolean;
}

export function Step(props: StepProps) {
  let prefix;
  if (props.stepNumber) {
    prefix = `Step ${props.stepNumber}: `;
  }

  return (
    <StepContainer>
      <Text bold>
        {prefix}
        {props.title}
      </Text>

      <TextSelectCopy text={props.text} mt={2} mb={1} bash={props.isBash} />
    </StepContainer>
  );
}

export const StepContainer = styled.div`
  background: ${props => props.theme.colors.spotBackground[0]};
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 12px;
`;
