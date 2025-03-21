import { matchPath } from 'react-router';

import { routePaths, type RouteRecord } from '../../config/routes';

const nonExactRoutes = [routePaths.accessGraph.base];

export const HistoryService = {
  canPush(route: string) {
    const knownRoutes = collectAllValues(routePaths);

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
    return this.canPush(url) ? url : routePaths.root;
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
