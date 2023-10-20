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
