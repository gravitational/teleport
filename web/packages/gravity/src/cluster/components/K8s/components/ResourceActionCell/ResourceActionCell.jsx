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
import MenuAction, { MenuItem } from 'gravity/cluster/components/components/ActionMenu';
import { Cell } from 'design/DataTable';
import { useK8sContext } from '../../k8sContext';

export default function ResourceActionCell({ rowIndex, data, children }) {
  const { resourceMap, name } = data[rowIndex];
  return (
    <Cell align="right">
      <ResourceActionCellMenu name={name} resourceMap={resourceMap} children={children} />
    </Cell>
  )
}

export function ResourceActionCellMenu({ name, resourceMap, children, ...rest }) {
  const { onViewResource } = useK8sContext();
  return (
    <MenuAction menuProps={menuProps} {...rest}>
      <MenuItem onClick={ () => onViewResource(name, resourceMap)}>
        Details
      </MenuItem>
      {children}
    </MenuAction>
  )
}

export {
  MenuItem
}

const menuProps = {
  anchorOrigin: {
    vertical: 'center',
    horizontal: 'center',
  },
  transformOrigin: {
    vertical: 'top',
    horizontal: 'center',
  },
}