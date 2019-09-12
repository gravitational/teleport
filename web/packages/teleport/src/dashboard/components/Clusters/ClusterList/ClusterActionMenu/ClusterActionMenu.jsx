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
