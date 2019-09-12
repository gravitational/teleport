import React from 'react';
import { Table, Cell, Column } from 'design/DataTable';
import { RoleNameCell, ActionCell, } from './RoleListCells';

const RoleList = ({ roles, onEdit, onDelete }) => {
  roles = roles || [];
  return (
    <Table data={roles}>
      <Column
        header={<Cell>Name</Cell> }
        cell={<RoleNameCell />}
        />
      <Column
        header={<Cell style={{textAlign: "right"}}>Actions</Cell>}
        cell={
          <ActionCell
            onEdit={onEdit}
            onDelete={onDelete} />
        }
      />
    </Table>
  )
};

export default RoleList;