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

import React from 'react';
import { Failed } from 'design/CardError';

import Logger from 'teleterm/logger';

export default class CatchError extends React.Component {
  logger = new Logger('components/CatchError');

  static getDerivedStateFromError(error) {
    return { error };
  }

  state = {
    error: null,
  };

  componentDidCatch(err) {
    this.logger.error('render', err);
  }

  render() {
    if (this.state.error) {
      return (
        <Failed alignSelf={'baseline'} message={this.state.error.message} />
      );
    }

    return this.props.children;
  }
}
