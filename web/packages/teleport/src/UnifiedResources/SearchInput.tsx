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

import React, { SetStateAction } from 'react';
import styled from 'styled-components';

import { height, space, color } from 'design/system';

// Taken from design.dataTable.InputSearch; will be modified later.
export function SearchInput({ searchValue, setSearchValue, children }: Props) {
  return (
    <WrapperBackground>
      <Wrapper>
        <StyledInput
          placeholder="Search for resources..."
          px={3}
          value={searchValue}
          onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
            setSearchValue(e.target.value)
          }
        />
        <ChildWrapperBackground>
          <ChildWrapper>{children}</ChildWrapper>
        </ChildWrapperBackground>
      </Wrapper>
    </WrapperBackground>
  );
}

type Props = {
  searchValue: string;
  setSearchValue: React.Dispatch<SetStateAction<string>>;
  children?: JSX.Element;
};

const ChildWrapper = styled.div`
  position: relative;
  height: 100%;
  right: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 0 200px 200px 0;
`;

const ChildWrapperBackground = styled.div`
  position: absolute;
  height: 100%;
  right: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-left: ${props => props.theme.borders[1]}
    ${props => props.theme.colors.spotBackground[0]};
  border-radius: 0 200px 200px 0;
`;

const Wrapper = styled.div`
  position: relative;
  display: flex;
  overflow: hidden;
  width: 100%;
  border-radius: 200px;
  height: 100%;
  background: transparent;
`;

const WrapperBackground = styled.div`
  background: ${props => props.theme.colors.levels.sunken};
  border-radius: 200px;
  width: 100%;
  height: ${props => props.theme.space[8]}px;
`;

const StyledInput = styled.input`
  border: none;
  outline: none;
  box-sizing: border-box;
  font-size: ${props => props.theme.fontSizes[3]}px;
  width: 100%;
  transition: all 0.2s;
  ${color}
  ${space}
  ${height}
  color: ${props => props.theme.colors.text.main};
  background: ${props => props.theme.colors.spotBackground[0]};
  padding-right: 184px;
  padding-left: ${props => props.theme.space[5]}px;
`;
