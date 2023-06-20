/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { forwardRef } from 'react';
import styled from 'styled-components';
import { Wand } from 'design/Icon';
import { Button } from 'design';

interface ConnectMyComputerIconProps {
  onClick(): void;
}

export const ConnectMyComputerIcon = forwardRef<
  HTMLDivElement,
  ConnectMyComputerIconProps
>((props, ref) => {
  return (
    <Container ref={ref}>
      <StyledButton
        onClick={props.onClick}
        kind="secondary"
        size="small"
        m="auto"
        title="Open Connect My Computer"
      >
        <Wand fontSize={16} />
      </StyledButton>
    </Container>
  );
});

const Container = styled.div`
  position: relative;
  display: inline-block;
`;

const StyledButton = styled(Button)`
  background: ${props => props.theme.colors.spotBackground[0]};
  padding: 9px;
  width: 30px;
  height: 30px;
`;
