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

import React from 'react';
import PropTypes from 'prop-types';
import styled from 'styled-components';

class MenuList extends React.Component {
  render() {
    const { children, ...other } = this.props;
    return (
      <StyledMenuList role="menu" {...other}>
        {children}
      </StyledMenuList>
    );
  }
}

const StyledMenuList = styled.div`
  background-color: ${props => props.theme.colors.levels.elevated};
  border-radius: 4px;
  box-shadow: ${props => props.theme.boxShadow[0]};
  box-sizing: border-box;
  max-height: calc(100% - 96px);
  overflow: hidden;
  position: relative;
  padding: 0;

  ${props => props.menuListCss && props.menuListCss(props)}
`;

MenuList.propTypes = {
  /**
   * MenuList contents, normally `MenuItem`s.
   */
  children: PropTypes.node,
  /**
   * @ignore
   */
  menuListCss: PropTypes.func,
};

export default MenuList;
