import React, { SetStateAction } from 'react';
import styled from 'styled-components';
import { height, space, color } from 'design/system';

export default function InputSearch({ searchValue, setSearchValue }: Props) {
  return (
    <Input
      placeholder="SEARCH..."
      px={3}
      value={searchValue}
      onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
        setSearchValue(e.target.value)
      }
    />
  );
}

type Props = {
  searchValue: string;
  setSearchValue: React.Dispatch<SetStateAction<string>>;
};

const Input = styled.input`
  box-sizing: border-box;
  font-size: 12px;
  min-width: 200px;
  outline: none;
  border: none;
  border-radius: 200px;
  height: 32px;
  transition: all .2s;
  ${fromTheme}
  ${space}
  ${color}
  ${height}
`;

function fromTheme(props) {
  return {
    color: props.theme.colors.text.primary,
    background: props.theme.colors.primary.dark,

    '&: hover, &:focus, &:active': {
      background: props.theme.colors.primary.main,
      boxShadow: 'inset 0 2px 4px rgba(0, 0, 0, .24)',
      color: props.theme.colors.text.primary,
    },
    '&::placeholder': {
      color: props.theme.colors.text.placeholder,
      fontSize: props.theme.fontSizes[1],
    },
  };
}
