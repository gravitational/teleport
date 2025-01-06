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

import { FormEvent, JSX } from 'react';
import styled from 'styled-components';

import {
  color,
  ColorProps,
  height,
  HeightProps,
  space,
  SpaceProps,
} from 'design/system';

const searchInputName = 'searchValue';

export default function InputSearch({
  searchValue,
  setSearchValue,
  children,
  bigInputSize = false,
}: Props) {
  function submitSearch(e: FormEvent<HTMLFormElement>) {
    e.preventDefault(); // prevent form default

    const formData = new FormData(e.currentTarget);
    const searchValue = formData.get(searchInputName) as string;

    setSearchValue(searchValue);
  }

  return (
    <WrapperBackground bigSize={bigInputSize}>
      <Form onSubmit={submitSearch}>
        <StyledInput
          bigInputSize={bigInputSize}
          placeholder="Search..."
          px={3}
          defaultValue={searchValue}
          name={searchInputName}
        />
        <ChildWrapperBackground>
          <ChildWrapper>{children}</ChildWrapper>
        </ChildWrapperBackground>
      </Form>
    </WrapperBackground>
  );
}

type Props = {
  searchValue: string;
  setSearchValue: (searchValue: string) => void;
  children?: JSX.Element;
  bigInputSize?: boolean;
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

const Form = styled.form`
  position: relative;
  display: flex;
  overflow: hidden;
  border-radius: 200px;
  height: 100%;
  background: transparent;
  max-width: 725px;
`;

const WrapperBackground = styled.div<{ bigSize: boolean }>`
  border-radius: 200px;
  width: 100%;
  height: ${props =>
    props.bigSize ? props.theme.space[7] : props.theme.space[6]}px;
`;

interface StyledInputProps extends ColorProps, SpaceProps, HeightProps {
  bigInputSize: boolean;
}

const StyledInput = styled.input<StyledInputProps>`
  border: none;
  outline: none;
  box-sizing: border-box;
  font-size: ${props =>
    props.bigInputSize ? props.theme.fontSizes[3] : props.theme.fontSizes[2]}px;
  width: 100%;
  transition: all 0.2s;
  ${color}
  ${space}
  ${height}
  color: ${props => props.theme.colors.text.main};
  background: ${props => props.theme.colors.spotBackground[0]};
  padding-right: 184px;
  // should match padding-left on StyledTable &:first-child to align Search content to Table content
  padding-left: ${props => props.theme.space[4]}px;
`;
