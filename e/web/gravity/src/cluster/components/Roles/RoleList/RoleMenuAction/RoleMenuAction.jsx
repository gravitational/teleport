import React from 'react';
import MenuAction, {
  MenuItem,
} from 'gravity/cluster/components/components/ActionMenu';

class RoleMenuAction extends React.Component {
  onEdit = () => {
    this.props.onEdit && this.props.onEdit(this.props.id);
  };

  onDelete = () => {
    this.props.onDelete && this.props.onDelete(this.props.id);
  };

  render() {
    return (
      <MenuAction>
        <MenuItem onClick={this.onEdit}>Edit...</MenuItem>
        <MenuItem onClick={this.onDelete}>Delete...</MenuItem>
      </MenuAction>
    );
  }
}

export default RoleMenuAction;
