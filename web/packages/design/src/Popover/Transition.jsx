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
import ReactDOM from 'react-dom';

class Transition extends React.Component {
  componentDidMount() {
    const node = ReactDOM.findDOMNode(this);
    this.props.onEntering(node);
  }

  render() {
    const { children, ...childProps } = this.props;
    delete childProps.onEntering;

    const child = React.Children.only(children);
    return React.cloneElement(child, childProps);
  }
}

export default Transition;
