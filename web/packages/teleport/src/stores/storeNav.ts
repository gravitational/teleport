/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { Store } from 'shared/libs/stores';

import { NavGroup, NavTitle } from 'teleport/types';

export const defaultNavState = {
  sideNav: [] as NavItem[],
  topNav: [] as NavItem[],
  topMenu: [] as NavItem[],
};

export default class StoreNav extends Store<typeof defaultNavState> {
  state = {
    ...defaultNavState,
  };

  addTopMenuItem(item: NavItem) {
    const items = [...this.state.topMenu, item];
    return this.setState({
      topMenu: items,
    });
  }

  addTopItem(item: NavItem) {
    const items = [...this.state.topNav, item];
    return this.setState({
      topNav: items,
    });
  }

  addSideItem(item: NavItem) {
    const items = [...this.state.sideNav, item];
    return this.setState({
      sideNav: items,
    });
  }

  getSideItems() {
    return this.state.sideNav;
  }

  getTopMenuItems() {
    return this.state.topMenu;
  }

  getTopItems() {
    return this.state.topNav;
  }
}

export type NavItem = {
  title: NavTitle;
  Icon: any;
  exact?: boolean;
  getLink(clusterId?: string): string;
  isExternalLink?: boolean;
  group?: NavGroup;
};
