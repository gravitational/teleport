/*
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

/**
 * LiveSessionAddressResolver is an address resolver that computes
 * a URL to start a new web-based SSH session.
 */
export default class LiveSessionAddressResolver {
  _cfg = {
    ttyUrl: null,
    ttyParams: {},
  };

  constructor(cfg) {
    this._cfg = {
      ...cfg,
    };
  }

  getConnStr(w, h) {
    const { ttyParams, ttyUrl } = this._cfg;
    const params = JSON.stringify({
      ...ttyParams,
      term: { h, w },
    });

    const encoded = window.encodeURI(params);
    return ttyUrl.replace(':params', encoded);
  }
}
