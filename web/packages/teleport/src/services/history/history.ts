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

import { createBrowserHistory } from 'history';

import { matchPath } from 'react-router';

import cfg from 'teleport/config';

import type { History } from 'history';

let _inst: History = null;

const history = {
  original() {
    return _inst;
  },

  init(history?: History) {
    _inst = history || createBrowserHistory();
  },

  replace(route = '') {
    route = this.ensureKnownRoute(route);
    _inst.replace(route);
  },

  push(route, withRefresh = false) {
    route = this.ensureKnownRoute(route);
    if (withRefresh) {
      this._pageRefresh(route);
    } else {
      _inst.push(route);
    }
  },

  reload() {
    window.location.reload();
  },

  goToLogin(rememberLocation = false) {
    let url = cfg.routes.login;
    if (rememberLocation) {
      const { search, pathname } = _inst.location;
      const knownRoute = this.ensureKnownRoute(pathname);
      const knownRedirect = this.ensureBaseUrl(knownRoute);
      const query = search ? encodeURIComponent(search) : '';

      url = `${url}?redirect_uri=${knownRedirect}${query}`;
    }

    this._pageRefresh(url);
  },

  getRedirectParam() {
    return getUrlParameter('redirect_uri', this.original().location.search);
  },

  ensureKnownRoute(route = '') {
    return this._canPush(route) ? route : cfg.routes.root;
  },

  ensureBaseUrl(url: string) {
    // create a URL object with url arg and optional `base` second arg set to cfg.baseUrl
    let urlWithBase = new URL(url || '', cfg.baseUrl);

    // if an attacker tries to pass a url such as teleport.example.com.bad.io
    // the cfg.baseUrl argument will be overridden. If it hasn't been overridden we can
    // assume that the passed url is either relative, or matches our cfg.baseUrl
    if (urlWithBase.origin !== cfg.baseUrl) {
      // create a new url with our base if the base doesn't match
      urlWithBase = new URL(urlWithBase.pathname, cfg.baseUrl);
    }

    return urlWithBase.toString();
  },

  getRoutes() {
    return Object.getOwnPropertyNames(cfg.routes).map(p => cfg.routes[p]);
  },

  getLocation() {
    return this.original().location;
  },

  _canPush(route: string) {
    const knownRoutes = this.getRoutes();
    const { pathname } = new URL(this.ensureBaseUrl(route));

    const match = (known: string) =>
      // only match against pathname
      matchPath(pathname, {
        path: known,
        exact: true,
      });

    return knownRoutes.some(match);
  },

  _pageRefresh(route: string) {
    window.location.href = this.ensureBaseUrl(route);
  },
};

export default history;

export function getUrlParameter(name = '', path = '') {
  const params = new URLSearchParams(path);
  const value = params.get(name);
  return value || '';
}
