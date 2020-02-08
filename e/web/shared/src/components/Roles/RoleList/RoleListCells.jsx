import React from 'react';
import { Cell } from 'design/DataTable';
import MenuAction, { MenuItem } from 'shared/components/ActionMenu';

export const RoleNameCell = ({ rowIndex, data }) => {
  const { displayName } = data[rowIndex];
  return <Cell>{displayName}</Cell>;
};

export const ActionCell = ({ rowIndex, onEdit, onDelete, data }) => {
  const { id, owner } = data[rowIndex];
  const onDeleteClick = () => onDelete(id);
  const onEditClick = () => onEdit(id);
  return (
    <Cell align="right">
      <MenuAction>
        <MenuItem onClick={onEditClick}>Edit...</MenuItem>
        <MenuItem disabled={owner} onClick={onDeleteClick}>
          Delete...
        </MenuItem>
      </MenuAction>
    </Cell>
  );
};
