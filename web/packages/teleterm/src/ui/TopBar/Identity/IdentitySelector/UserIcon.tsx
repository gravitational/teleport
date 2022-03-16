import React from 'react';
import styled from 'styled-components';

interface UserIconProps {
  letter: string;
}

export function UserIcon(props: UserIconProps) {
  return <Circle>{props.letter.toLocaleUpperCase()}</Circle>
}

const Circle = styled.span`
  border-radius: 50%;  
  color: ${props => props.theme.colors.light};
  background: ${props => props.theme.colors.secondary.main};
  height: 24px;
  width: 24px;
  display: flex;
  flex-shrink: 0;
  justify-content: center;
  align-items: center;
  overflow: hidden;
`