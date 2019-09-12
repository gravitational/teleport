import React from 'react';
import { SiteStateEnum } from 'gravity/services/enums';
import MenuAction, { MenuItem } from 'gravity/cluster/components/components/ActionMenu';
import { useHubClusterContext } from './../hubClusterStore';

export default function ClusterActionMenu({ children, cluster, ...rest }) {
  const url = cluster.state === SiteStateEnum.INSTALLING ? cluster.installerUrl : cluster.siteUrl;
  const { setClusterToDisconnect } = useHubClusterContext();
  return (
    <MenuAction buttonIconProps={{ style: { marginLeft: "auto" }}} menuProps={menuProps} {...rest}>
      <MenuItem as="a" href={url} target="_blank">
        View
      </MenuItem>
      <MenuItem onClick={ () => setClusterToDisconnect(cluster)}>
        Remove from Ops Center...
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