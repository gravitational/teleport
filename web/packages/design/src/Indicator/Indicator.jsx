/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import styled from 'styled-components';

import PropTypes from 'prop-types';

import { Spinner as SpinnerIcon } from './../Icon';

const DelayValueMap = {
  none: 0,
  short: 400, // 0.4s;
  long: 600, // 0.6s;
};

class Indicator extends React.Component {
  constructor(props) {
    super(props);
    this._timer = null;
    this._delay = props.delay;
    this.state = {
      canDisplay: false,
    };
  }

  componentDidMount() {
    let timeoutValue = DelayValueMap[this._delay];
    this._timer = setTimeout(() => {
      this.setState({
        canDisplay: true,
      });
    }, timeoutValue);
  }

  componentWillUnmount() {
    clearTimeout(this._timer);
  }

  render() {
    if (!this.state.canDisplay) {
      return null;
    }

    return <StyledSpinner {...this.props} />;
  }
}

Indicator.propTypes = {
  delay: PropTypes.oneOf(['none', 'short', 'long']),
};

Indicator.defaultProps = {
  delay: 'short',
};

const StyledSpinner = styled(SpinnerIcon)`
  ${({ fontSize = '32px' }) => `
    font-size: ${fontSize};
    height: ${fontSize};
    width: ${fontSize};
  `}

  animation: anim-rotate 2s infinite linear;
  color: ${props => props.theme.colors.spotBackground[0]}
  display: inline-block;
  margin: 16px;
  opacity: 0.24;

  @keyframes anim-rotate {
    0% {
      transform: rotate(0);
    }
    100% {
      transform: rotate(360deg);
    }
  }
`;

export default Indicator;
