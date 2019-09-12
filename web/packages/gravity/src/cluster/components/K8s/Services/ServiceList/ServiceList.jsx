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
import { Table, Column, Cell, TextCell } from 'design/DataTable';
import ResourceActionCell  from './../../components/ResourceActionCell';
import { NameCell, PortCell, LabelCell, } from './ServiceListCells';

function ServiceList(props) {
  const { namespace, services } = props;
  const filtered = services.filter( item => item.namespace === namespace );
  return (
    <Table data={filtered}>
      <Column
        header={<Cell>Name</Cell> }
        cell={<NameCell /> }
      />
      <Column
        columnKey="clusterIp"
        header={<Cell>Cluster</Cell> }
        cell={<TextCell/> }
      />
      <Column
        header={<Cell>Ports</Cell> }
        cell={<PortCell/> }
      />
      <Column
        header={<Cell>Labels</Cell> }
        cell={<LabelCell/> }
      />
      <Column
        header={<Cell></Cell> }
        cell={<ResourceActionCell/>}
      />
    </Table>
  )
}

export default ServiceList;