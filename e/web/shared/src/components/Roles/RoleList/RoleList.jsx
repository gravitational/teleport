import React from 'react';
import { Table, Cell, Column } from 'design/DataTable';
import { RoleNameCell, ActionCell } from './RoleListCells';

const RoleList = ({ items, onEdit, onDelete }) => {
  items = items || [];
  return (
    <Table data={items}>
      <Column header={<Cell>Name</Cell>} cell={<RoleNameCell />} />
      <Column
        header={<Cell />}
        cell={<ActionCell onEdit={onEdit} onDelete={onDelete} />}
      />
    </Table>
  );
};

export default RoleList;
