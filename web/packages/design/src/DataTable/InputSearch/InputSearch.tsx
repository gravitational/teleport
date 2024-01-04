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

import React, { SetStateAction } from 'react';
import styled from 'styled-components';

import { height, space, color } from 'design/system';

export default function InputSearch({
  searchValue,
  setSearchValue,
  children,
}: Props) {
  return (
    <WrapperBackground>
      <Wrapper>
        <StyledInput
          placeholder="SEARCH..."
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
  background: ${props => props.theme.colors.spotBackground[0]};
  border-radius: 200px;
`;

const ChildWrapperBackground = styled.div`
  position: absolute;
  height: 100%;
  right: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: ${props =>
    props.theme.type === 'dark'
      ? props.theme.colors.levels.surface
      : props.theme.colors.levels.deep};
  border-radius: 200px;
`;

const Wrapper = styled.div`
  position: relative;
  display: flex;
  overflow: hidden;
  width: 100%;
  border-radius: 200px;
  height: 32px;
  background: transparent;
`;

const WrapperBackground = styled.div`
  background: ${props => props.theme.colors.levels.sunken};
  border-radius: 200px;
  width: 100%;
  height: 32px;
`;

const StyledInput = styled.input`
  border: none;
  outline: none;
  box-sizing: border-box;
  height: 100%;
  font-size: 12px;
  width: 100%;
  transition: all 0.2s;
  ${color}
  ${space}
  ${height}
  ${fromTheme};
  padding-right: 184px;
`;

function fromTheme(props) {
  return {
    color: props.theme.colors.text.main,
    background:
      props.theme.type === 'dark'
        ? props.theme.colors.levels.sunken
        : props.theme.colors.levels.deep,

    '&:hover, &:focus, &:active': {
      color: props.theme.colors.text.main,
      background:
        props.theme.type === 'dark'
          ? props.theme.colors.spotBackground[0]
          : props.theme.colors.levels.sunken,
    },
    '&::placeholder': {
      color: props.theme.colors.text.muted,
      fontSize: props.theme.fontSizes[1],
    },
  };
}
