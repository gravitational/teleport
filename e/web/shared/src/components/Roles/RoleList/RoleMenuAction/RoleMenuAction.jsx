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
