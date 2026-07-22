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

import { Component } from 'react';

const DEFAULT_INTERVAL = 3000; // every 3 sec

export default class AjaxPoller extends Component {
  _timerId = null;
  _request = null;

  static defaultProps = {
    immediately: true,
  };

  constructor(props) {
    super(props);
    this._intervalTime = props.time || DEFAULT_INTERVAL;
  }

  fetch() {
    // do not refetch if still in progress
    if (this._request) {
      return;
    }

    this._request = this.props.onFetch().finally(() => {
      this._request = null;
    });
  }

  componentDidMount() {
    this.props.immediately && this.fetch();
    this._timerId = setInterval(this.fetch.bind(this), this._intervalTime);
  }

  componentWillUnmount() {
    clearInterval(this._timerId);
    if (this._request && this._request.abort) {
      this._request.abort();
    }
  }

  render() {
    return null;
  }
}
