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
import { Table, Column, Cell, TextCell } from 'design/DataTable';
import { NameCell, DesiredCell, StatusCell } from './JobListCells';
import ResourceActionCell  from './../../components/ResourceActionCell';

function JobsTab(props) {
  let { jobs, namespace } = props;
  jobs = jobs.filter( item => item.namespace === namespace );
  jobs = sortBy(jobs, ['created']).reverse();

  return (
    <Table data={jobs}>
      <Column
        header={<Cell>Name</Cell> }
        cell={<NameCell/> }
      />
      <Column
        header={<Cell>Desired</Cell> }
        cell={<DesiredCell/> }
      />
      <Column
        header={<Cell>Status</Cell> }
        cell={<StatusCell/> }
      />
      <Column
        columnKey="createdDisplay"
        header={<Cell>Age</Cell> }
        cell={<TextCell/> }
      />
      <Column
        header={<Cell></Cell> }
        cell={<ResourceActionCell/>}
      />
    </Table>
  )
}

export default JobsTab;