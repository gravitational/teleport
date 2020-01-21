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
import MenuAction, {
  MenuItem,
} from 'gravity/cluster/components/components/ActionMenu';

class UserMenuAction extends React.Component {
  onDelete = () => {
    this.props.onDelete(this.props.userId);
  };

  onEdit = () => {
    this.props.onEdit(this.props.userId);
  };

  onReset = () => {
    this.props.onReset(this.props.userId);
  };

  onDelete = () => {
    this.props.onDelete(this.props.userId);
  };

  render() {
    const { isInvite, owner } = this.props;
    return <MenuAction>{this.renderItems(isInvite, owner)}</MenuAction>;
  }

  // returns an array so MenuAction component can pass down it's properties
  renderItems(isInvite, isOwner) {
    if (!isInvite) {
      return [
        <MenuItem key="1" onClick={this.onEdit}>
          Edit
        </MenuItem>,
        <MenuItem key="2" onClick={this.onReset}>
          Reset Password
        </MenuItem>,
        <MenuItem key="3" disabled={isOwner} onClick={this.onDelete}>
          Remove User
        </MenuItem>,
      ];
    }

    return [
      <MenuItem key="1" onClick={this.onDelete}>
        Revoke invitation...
      </MenuItem>,
    ];
  }
}

export default UserMenuAction;
