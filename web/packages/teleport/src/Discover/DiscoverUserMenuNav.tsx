import React from 'react';

import useTeleport from 'teleport/useTeleport';

import { UserMenuNav } from 'teleport/components/UserMenuNav';

interface DiscoverUserMenuNavProps {
  logout: () => void;
}

export function DiscoverUserMenuNav(props: DiscoverUserMenuNavProps) {
  const ctx = useTeleport();

  return (
    <UserMenuNav
      navItems={ctx.storeNav.getTopMenuItems()}
      logout={props.logout}
      username={ctx.storeUser.getUsername()}
    />
  );
}
