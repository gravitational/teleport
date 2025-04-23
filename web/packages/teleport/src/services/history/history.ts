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

import { createBrowserHistory, type History } from 'history';
import { matchPath } from 'react-router';

import cfg from 'teleport/config';

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

  goToLogin({
    rememberLocation = false,
    withAccessChangedMessage = false,
  } = {}) {
    const params: string[] = [];

    // withAccessChangedMessage determines whether the login page the user is redirected to should include a notice that
    // they were logged out due to their roles having changed.
    if (withAccessChangedMessage) {
      params.push('access_changed');
    }

    if (rememberLocation) {
      const { search, pathname } = _inst.location;
      const knownRoute = this.ensureKnownRoute(pathname);
      const knownRedirect = this.ensureBaseUrl(knownRoute);
      const query = search ? encodeURIComponent(search) : '';
      params.push(`redirect_uri=${knownRedirect}${query}`);
    }

    const queryString = params.join('&');
    const url = queryString
      ? `${cfg.routes.login}?${queryString}`
      : cfg.routes.login;

    this._pageRefresh(url);
  },

  // TODO (avatus): make this return a path only if a full URI is present
  getRedirectParam() {
    return getUrlParameter('redirect_uri', this.original().location.search);
  },

  hasAccessChangedParam() {
    return hasUrlParameter('access_changed', this.original().location.search);
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
    return collectAllValues(cfg.routes);
  },

  getLocation() {
    return this.original().location;
  },

  _canPush(route: string) {
    const knownRoutes = this.getRoutes();
    const nonExactRoutes = cfg.getNonExactRoutes();

    const { pathname } = new URL(this.ensureBaseUrl(route));

    const match = (known: string) =>
      // only match against pathname
      matchPath(pathname, {
        path: known,
        exact: !nonExactRoutes.includes(known),
      });

    return knownRoutes.some(match);
  },

  _pageRefresh(route: string) {
    window.location.href = this.ensureBaseUrl(route);
  },
};

interface RouteRecord {
  [key: string]: string | RouteRecord;
}

function collectAllValues(record: RouteRecord) {
  const result: string[] = [];

  for (const key in record) {
    if (typeof record[key] === 'string') {
      result.push(record[key]);
    } else {
      result.push(...collectAllValues(record[key]));
    }
  }

  return result;
}

export default history;

export function getUrlParameter(name = '', path = '') {
  const params = new URLSearchParams(path);
  const value = params.get(name);
  return value || '';
}

function hasUrlParameter(name = '', path = '') {
  const params = new URLSearchParams(path);
  return params.has(name);
}
