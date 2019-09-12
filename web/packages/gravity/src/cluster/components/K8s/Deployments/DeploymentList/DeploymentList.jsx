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
import { sortBy } from 'lodash';
import { Table, TextCell, Column, Cell } from 'design/DataTable';
import ResourceActionCell  from './../../components/ResourceActionCell';

function DeploymentList(props) {
  let { deployments, namespace } = props;
  deployments = deployments.filter(item => item.namespace === namespace);
  deployments = sortBy(deployments, ['created']).reverse();
  return (
    <Table data={deployments}>
      <Column
        columnKey="name"
        header={<Cell >Name</Cell>}
        cell={<TextCell />}
      />
      <Column
        columnKey="desired"
        header={<Cell>Desired</Cell>}
        cell={<TextCell />}
      />
      <Column
        columnKey="statusCurrentReplicas"
        header={<Cell>Current</Cell>}
        cell={<TextCell />}
      />
      <Column
        columnKey="statusUpdatedReplicas"
        header={<Cell>Up-to-date</Cell>}
        cell={<TextCell />}
      />
      <Column
        columnKey="statusAvailableReplicas"
        header={<Cell>Available</Cell>}
        cell={<TextCell />}
      />
      <Column
        columnKey="createdDisplay"
        header={<Cell>Age</Cell>}
        cell={<TextCell />}
      />
      <Column
        header={<Cell></Cell> }
        cell={<ResourceActionCell/>}
      />
    </Table>
  )
}

export default DeploymentList;