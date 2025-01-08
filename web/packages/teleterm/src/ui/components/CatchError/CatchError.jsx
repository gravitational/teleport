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

import { UnhandledCaseError } from 'shared/utils/assertUnreachable';

import Logger from 'teleterm/logger';
import { FailedApp } from 'teleterm/ui/components/App';

export class CatchError extends Component {
  logger = new Logger('CatchError');

  static getDerivedStateFromError(error) {
    return { error };
  }

  state = {
    error: null,
  };

  componentDidCatch(err) {
    if (err instanceof UnhandledCaseError) {
      try {
        // Log this message only to console to avoid storing it on disk. unhandledCase might be an
        // object which holds sensitive data.
        console.error('Unhandled case', JSON.stringify(err.unhandledCase));
      } catch (error) {
        console.error('Cannot stringify unhandled case', error);
      }
    }

    this.logger.error('Caught', err.stack || err);
  }

  render() {
    if (this.state.error) {
      return (
        <FailedApp message={this.state.error?.message || this.state.error} />
      );
    }

    return this.props.children;
  }
}
