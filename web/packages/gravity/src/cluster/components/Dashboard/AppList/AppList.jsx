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
import { values, sortBy } from 'lodash';
import { TablePaged, Column, Cell, TextCell } from 'design/DataTable';
import { EndpointCell, StatusCell, AppNameCell } from './AppListCells';
import { Flex, Text, Box } from 'design';
import { withState } from 'shared/hooks';
import { useFluxStore } from 'gravity/components/nuclear';
import { getters } from 'gravity/cluster/flux/apps';

export function AppList(props){
  const { apps, pageSize=5,  ...rest } = props;
  const data = sortBy(values(apps), 'updated').reverse();
  return (
    <Box {...rest}>
      <Flex bg="primary.light" px="3" py="2" alignItems="center" borderTopRightRadius="3" borderTopLeftRadius="3">
        <Text typography="h4">
          Installed Applications
        </Text>
      </Flex>
      <TablePaged data={data} pageSize={pageSize}>
        <Column
          header={
            <Cell>Status</Cell>
          }
          cell={<StatusCell/> }
        />
        <Column
          header={
            <Cell>Name</Cell>
          }
          cell={<AppNameCell/> }
        />
        <Column
          columnKey="chartVersion"
          header={
            <Cell>Version</Cell>
          }
          cell={<TextCell/> }
        />
        <Column header={
            <Cell>Endpoints</Cell>
          }
          cell={<EndpointCell/> }
        />

        <Column
          columnKey="updatedText"
          header={
            <Cell style={alignedRight}>Updated</Cell>
          }
          cell={<TextCell style={alignedRight}/> }
        />
      </TablePaged>
    </Box>
  )
}

const alignedRight = {
  textAlign: "right",
  minWidth: '140px'
}

const mapState = () => {
  const appStore = useFluxStore(getters.appStore);
  return {
    apps: appStore.apps
  }
}

export default withState(mapState)(AppList);