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

class RootRef extends React.Component {
  componentDidMount() {
    this.ref = ReactDOM.findDOMNode(this);
    setRef(this.props.rootRef, this.ref);
  }

  componentDidUpdate(prevProps) {
    const ref = ReactDOM.findDOMNode(this);

    if (prevProps.rootRef !== this.props.rootRef || this.ref !== ref) {
      if (prevProps.rootRef !== this.props.rootRef) {
        setRef(prevProps.rootRef, null);
      }

      this.ref = ref;
      setRef(this.props.rootRef, this.ref);
    }
  }

  componentWillUnmount() {
    this.ref = null;
    setRef(this.props.rootRef, null);
  }

  render() {
    return this.props.children;
  }
}

function setRef(ref, value) {
  if (typeof ref === 'function') {
    ref(value);
  } else if (ref) {
    ref.current = value;
  }
}

RootRef.propTypes = {
  /**
   * The wrapped element.
   */
  children: PropTypes.element.isRequired,
  /**
   * Provide a way to access the DOM node of the wrapped element.
   * You can provide a callback ref or a `React.createRef()` ref.
   */
  rootRef: PropTypes.oneOfType([PropTypes.func, PropTypes.object]).isRequired,
};

export default RootRef;
