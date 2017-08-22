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
import classnames from 'classnames';
const SSH_STR_REGEX = /(^\w+\@(\w|\.|\-)+(:\d+)*$)|(^$)/;
const PLACEHOLDER_TEXT = 'login@host';

export default class InputSshServer extends React.Component {

  state = {
    hasErrors: false
  }
  
  onChange = e => {
    const value = e.target.value;
    const isValid = this.isValid(value);
    if (isValid && this.state.hasErrors === true) {
      this.setState({hasErrors: false})      
    }    
  }

  onKeyPress = e => {
    const value = e.target.value;
    const isValid = this.isValid(value);
    if ((e.key === 'Enter' || e.type === 'click') && value) {                  
      this.setState({ hasErrors: !isValid })      
      if (isValid) {
        const [login, host] = value.split('@');
        this.props.onEnter(login, host);              
      }      
    }    
  }
  
  isValid(value) {    
    const match = SSH_STR_REGEX.exec(value);
    return !!match;
  }

  render() {    
    const className = classnames('grv-sshserver-input', { '--error': this.state.hasErrors });
    return (
      <div className={className} >
        <div className="m-l input-group input-group-sm" title="login to SSH server">
          <input className="form-control"            
            placeholder={PLACEHOLDER_TEXT}
            onChange={this.onChange}
            onKeyPress={this.onKeyPress}
          />
          <span className="input-group-btn">
            <button className="btn btn-sm btn-white" onClick={this.onKeyPress}>
              <i className="fa fa-terminal text-muted" />
            </button>
          </span>
        </div>
        <label className="m-l grv-sshserver-input-errors"> Invalid format </label>
      </div>
    )  
  }
}

