import { matchPath } from 'react-router';

import { cfg } from '../../config';
import type { RouteRecord } from '../../config/routes';

const nonExactRoutes = [cfg.routes.accessGraph.base];

export const HistoryService = {
  canPush(route: string) {
    const knownRoutes = collectAllValues(cfg.routes);

    const { pathname } = new URL(this.ensureBaseUrl(route));

    const match = (known: string) =>
      // only match against pathname
      matchPath(pathname, {
        path: known,
        exact: !nonExactRoutes.includes(known),
      });

    return knownRoutes.some(match);
  },
  ensureBaseUrl(url: string) {
    const baseUrl = window.location.origin;

    // create a URL object with url arg and optional `base` second arg set to cfg.baseUrl
    let urlWithBase = new URL(url || '', baseUrl);

    // if an attacker tries to pass a url such as teleport.example.com.bad.io
    // the cfg.baseUrl argument will be overridden. If it hasn't been overridden we can
    // assume that the passed url is either relative, or matches our cfg.baseUrl
    if (urlWithBase.origin !== baseUrl) {
      // create a new url with our base if the base doesn't match
      urlWithBase = new URL(urlWithBase.pathname, baseUrl);
    }

    return urlWithBase.toString();
  },
  ensureKnownRoute(url: string) {
    return this.canPush(url) ? url : cfg.routes.root;
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
      const { search, pathname } = window.location;

      const knownRoute = this.ensureKnownRoute(pathname);
      const knownRedirect = this.ensureBaseUrl(knownRoute);

      const query = search ? encodeURIComponent(search) : '';

      params.push(`redirect_uri=${knownRedirect}${query}`);
    }

    const queryString = params.join('&');
    const url = queryString
      ? `${cfg.routes.login}?${queryString}`
      : cfg.routes.login;

    this.refreshPage(url);
  },
  refreshPage(url: string) {
    window.location.href = this.ensureBaseUrl(url);
  },
};

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
