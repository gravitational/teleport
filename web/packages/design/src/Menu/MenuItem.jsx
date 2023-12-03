/*
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

import PropTypes from 'prop-types';
import styled from 'styled-components';
import { fontSize, color, space } from 'styled-system';

const defaultValues = {
  fontSize: 1,
  px: 3,
};

const fromTheme = props => {
  const values = {
    ...defaultValues,
    ...props,
  };
  return {
    ...fontSize(values),
    ...space(values),
    ...color(values),
    fontWeight: values.theme.regular,

    '&:hover, &:focus': {
      color: props.disabled
        ? values.theme.colors.text.disabled
        : values.theme.colors.text.main,
      background: values.theme.colors.spotBackground[0],
    },
    '&:active': {
      background: values.theme.colors.spotBackground[1],
    },
  };
};

const MenuItem = styled.div`
  min-height: 40px;
  box-sizing: border-box;
  cursor: ${props => (props.disabled ? 'not-allowed' : 'pointer')};
  display: flex;
  justify-content: flex-start;
  align-items: center;
  min-width: 140px;
  overflow: hidden;
  text-decoration: none;
  white-space: nowrap;
  color: ${props =>
    props.disabled
      ? props.theme.colors.text.disabled
      : props.theme.colors.text.main};

  ${fromTheme}
`;

MenuItem.displayName = 'MenuItem';
MenuItem.propTypes = {
  /**
   * Menu item contents.
   */
  children: PropTypes.node,
};

export default MenuItem;
