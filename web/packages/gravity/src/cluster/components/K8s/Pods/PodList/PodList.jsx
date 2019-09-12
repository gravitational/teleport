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
import { Table, Column, Cell } from 'design/DataTable';
import isMatch from 'design/utils/match';
import { ActionCell, NameCell, StatusCell, ContainerCell, LabelCell } from './PodListCells';

const NAMESPACE_KEY = 'namespace';

class PodList extends React.Component {

  constructor(props) {
    super(props);
    this.searchableComplexProps = ['containerNames', 'labelsText'];
    this.searchableProps = ['podHostIp', 'podIp', 'name', 'phaseValue', ...this.searchableComplexProps];
  }

  searchAndFilterCb = (targetValue, searchValue, propName) => {
    if(this.searchableComplexProps.indexOf(propName)!== -1){
      return targetValue.some((item) => {
        return item.toLocaleUpperCase().indexOf(searchValue) !==-1;
      });
    }
  }

  sortAndFilter(data){
    const { namespace, searchValue='' } = this.props;
    const filtered = data
    .filter( item => item[NAMESPACE_KEY] === namespace )
    .filter( obj=> isMatch(obj, searchValue, {
        searchableProps: this.searchableProps,
        cb: this.searchAndFilterCb
      }));

    return filtered;
  }

  render() {
    const { sshLogins, logsEnabled, monitoringEnabled, podInfos } = this.props;
    const data = this.sortAndFilter(podInfos);
    return (
      <Table data={data}>
        <Column
          header={<Cell>Name</Cell> }
          cell={<NameCell /> }
        />
        <Column
          header={<Cell>Status</Cell> }
          cell={<StatusCell/>}
        />
        <Column
          header={<Cell>Containers</Cell> }
          cell={<ContainerCell sshLogins={sshLogins} logsEnabled={logsEnabled}/>}
        />
        <Column
          header={<Cell>Labels</Cell> }
          cell={<LabelCell/>}
          />
        <Column
          header={<Cell></Cell> }
          cell={<ActionCell logsEnabled={logsEnabled} monitoringEnabled={monitoringEnabled}/>}
        />
      </Table>
    )
  }
}

export default PodList;