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

import React, { PropTypes } from 'react';
import classnames from 'classnames';

const SSH_STR_REGEX = /(^\w+\@(\w|\.|\-)+(:\d+)*$)|(^$)/;
const PLACEHOLDER_TEXT = 'login@host';
const DEFAULT_HISTORY_INDEX = -1; 

const KeyEnum = {  
  UP: 38,  
  DOWN: 40
}

export default class InputSshServer extends React.Component {

  prevLoginIndex = DEFAULT_HISTORY_INDEX

  static propTypes = {    
    sshHistory: PropTypes.object.isRequired,
    clusterId: PropTypes.string.isRequired,
    onEnter:  PropTypes.func.isRequired,
  } 

  state = {    
    hasErrors: false,
    value: ''
  }

  setDefaultPrevIndex() {
    this.prevLoginIndex = DEFAULT_HISTORY_INDEX;
  }
  
  componentWillReceiveProps(nextProps) {    
    if (nextProps.clusterId !== this.props.clusterId || 
        nextProps.sshHistory !== this.props.sshHistory) {      
      this.setDefaultPrevIndex();
      this.setState({value: ''})
    }        
  }
  
  onChange = e => {
    const value = e.target.value;
    const isValid = this.isValid(value);
    if (isValid && this.state.hasErrors === true) {
      this.setState({
        hasErrors: false,
        value
      })      
    }    

    this.setState({ value });
  }

  getPrevLogins() {
    const { sshHistory, clusterId } = this.props;
    return sshHistory.getPrevLogins(clusterId);    
  }

  getNextLogin(dir) {        
    const logins = this.getPrevLogins();

    if (logins.length === 0) {
      return '';
    }
    
    let index = this.prevLoginIndex + dir;   
    
    if (index < 0) {
      this.setDefaultPrevIndex();
      return '';
    } 

    if (index >= logins.length) {
      index = this.prevLoginIndex;
    } else {
      this.prevLoginIndex = index;  
    }
    
    return logins[this.prevLoginIndex];
  }

  onKeyUp = e => {            
    if (this.getPrevLogins().length === 0) {
      return;
    }

    let dir = 0;
    const keyCode = e.which;            
    if (keyCode === KeyEnum.UP) {
      dir = 1;
    }

    if (keyCode === KeyEnum.DOWN) {
      dir = -1;
    }
    
    if (dir === 0) {
      return;
    }
        
    const login = this.getNextLogin(dir);
    this.setState({ value: login });    
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
    const { value, hasErrors } = this.state;
    const { autoFocus = false } = this.props;
    const className = classnames('grv-sshserver-input', { '--error': hasErrors });
    return (
      <div className={className} >
        <div className="m-l input-group-sm" title="login to SSH server">
          <input ref={e => { this.inputRef = e }} className="form-control"                
            placeholder={PLACEHOLDER_TEXT}
            value={value}
            autoFocus={autoFocus}
            onChange={this.onChange}
            onKeyUp={this.onKeyUp}
            onKeyPress={this.onKeyPress}
          />          
        </div>
        <label className="m-l grv-sshserver-input-errors"> Invalid format </label>
      </div>
    )  
  }
}