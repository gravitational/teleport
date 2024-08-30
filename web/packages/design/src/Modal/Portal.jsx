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
import PropTypes from 'prop-types';

/**
 * Portals provide a first-class way to render children into a DOM node
 * that exists outside the DOM hierarchy of the parent component.
 */
class Portal extends React.Component {
  componentDidMount() {
    this.setMountNode(this.props.container);

    // Portal initializes the container and mounts it to the DOM during
    // first render. No children are rendered at this time.
    // ForceUpdate is called to render children elements inside
    // the container after it gets mounted.
    this.forceUpdate();
  }

  componentDidUpdate(prevProps) {
    if (prevProps.container !== this.props.container) {
      this.setMountNode(this.props.container);

      this.forceUpdate();
    }
  }

  componentWillUnmount() {
    this.mountNode = null;
  }

  setMountNode(container) {
    this.mountNode = getContainer(container, document.body);
  }

  /**
   * @public
   */
  getMountNode = () => {
    return this.mountNode;
  };

  render() {
    const { children } = this.props;

    return this.mountNode
      ? ReactDOM.createPortal(children, this.mountNode)
      : null;
  }
}

Portal.propTypes = {
  /**
   * The children to render into the `container`.
   */
  children: PropTypes.node.isRequired,
  /**
   * A node, component instance, or function that returns either.
   * The `container` will have the portal children appended to it.
   * By default, it uses the body of the top-level document object,
   * so it's simply `document.body` most of the time.
   */
  container: PropTypes.oneOfType([PropTypes.object, PropTypes.func]),
};

function getContainer(container, defaultContainer) {
  container = typeof container === 'function' ? container() : container;
  return ReactDOM.findDOMNode(container) || defaultContainer;
}

export default Portal;
