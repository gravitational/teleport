/*
Copyright 2019-2020 Gravitational, Inc.

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
import MenuAction, { MenuItem } from 'teleport/components/ActionMenu';

export default function ClusterActionMenu({ children, cluster, ...rest }) {
  const { url } = cluster;
  return (
    <MenuAction
      buttonIconProps={{ style: { marginLeft: 'auto' } }}
      menuProps={menuProps}
      {...rest}
    >
      <MenuItem as="a" href={url} target="_blank">
        View
      </MenuItem>
      {children}
    </MenuAction>
  );
}

export { MenuItem };

const menuProps = {
  anchorOrigin: {
    vertical: 'center',
    horizontal: 'center',
  },
  transformOrigin: {
    vertical: 'top',
    horizontal: 'center',
  },
};
