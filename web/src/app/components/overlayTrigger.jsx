/*
Copyright 2015 Gravitational, Inc.

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
import ReactDOM from 'react-dom';
import { Overlay } from 'react-overlays'

const triggerType = React.PropTypes.oneOf(['click', 'hover', 'focus']);

const propTypes = {
    
  trigger: React.PropTypes.oneOfType([
    triggerType, React.PropTypes.arrayOf(triggerType),
  ]),
  
  delay: React.PropTypes.number,
  
  delayShow: React.PropTypes.number,
  
  delayHide: React.PropTypes.number,
  
  defaultOverlayShown: React.PropTypes.bool,

  overlay: React.PropTypes.node.isRequired,
  
  onBlur: React.PropTypes.func,

  onClick: React.PropTypes.func,

  onFocus: React.PropTypes.func,

  onMouseOut: React.PropTypes.func,

  onMouseOver: React.PropTypes.func,

  target: React.PropTypes.oneOf([null]),

  onHide: React.PropTypes.oneOf([null]),

  show: React.PropTypes.oneOf([null]),
};

const defaultProps = {
  defaultOverlayShown: false,
  trigger: ['hover', 'focus'],
};

class OverlayTrigger extends React.Component {
  constructor(props, context) {
    super(props, context);
    this.getElement = this.getElement.bind(this);
    this.handleToggle = this.handleToggle.bind(this);    
    this.handleHide = this.handleHide.bind(this);    
    this.state = {
      show: props.defaultOverlayShown,
    };
  }

  handleToggle() {
    if (this.state.show) {
      this.hide();
    } else {
      this.show();
    }
  }
    
  handleHide() {
    this.hide();
  }

  show() {
    this.setState({ show: true });
  }

  hide() {
    this.setState({ show: false });
  }
 
  getElement() {
    return ReactDOM.findDOMNode(this);    
  }

  render() {
    let { container = this, placement, overlay } = this.props;                                                
    return (    
      <div onClick={this.handleToggle}>
        {this.props.children}
        <Overlay
          rootClose={true}
          placement={placement}
          show={this.state.show}
          onHide={this.handleHide}
          target={ () => this.getElement()}
          container={container} >  
          {overlay}  
        </Overlay>  
      </div>  
    )    
  }
}

OverlayTrigger.propTypes = propTypes;
OverlayTrigger.defaultProps = defaultProps;

export default OverlayTrigger;