/*
Copyright 2019-2020 Gravitational, Inc.

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
import { MenuIcon, MenuItem } from 'shared/components/MenuAction';

class RoleMenuAction extends React.Component {
  onEdit = () => {
    this.props.onEdit && this.props.onEdit(this.props.id);
  };

  onDelete = () => {
    this.props.onDelete && this.props.onDelete(this.props.id);
  };

  render() {
    return (
      <MenuIcon>
        <MenuItem onClick={this.onEdit}>Edit...</MenuItem>
        <MenuItem onClick={this.onDelete}>Delete...</MenuItem>
      </MenuIcon>
    );
  }
}

export default RoleMenuAction;
