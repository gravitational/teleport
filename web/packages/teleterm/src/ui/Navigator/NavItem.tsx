/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import styled from 'styled-components';
import { color, space } from 'design/system';

// type Props = {
//   active: boolean;
//   onClick?: () => void;
// };
//
// const NavItem: React.FC<Props> = props => {
//   const { onClick } = props;
//   return (
//     <ListItem onClick={onClick}>
//       {props.children}
//     </ListItem>
//   );
// };

export const ListItem = styled.button`
  white-space: nowrap;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  justify-content: flex-start;
  cursor: pointer;
  width: 100%;
  position: relative;
  font-size: 14px;
  padding: 0 16px;
  font-weight: ${props => props.theme.regular};
  font-family: ${props => props.theme.font};
  color: ${props => props.theme.colors.text.primary};
  height: 36px;
  background: inherit;
  border: none;

  &:hover {
    background: ${props => props.theme.colors.primary.light};
  }

  &:focus, &:hover {
    color: ${props => props.theme.colors.primary.contrastText};
  }
,
`;

const StyledNavItem = styled.div(props => {
  const { theme, $active } = props;
  const colors = $active
    ? {
        color: theme.colors.primary.contrastText,
      }
    : {};

  return {
    ...colors,
    ...color(props),
    ...space(props),
  };
});

// export default NavItem;
