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
import $ from 'jQuery';
import { isObject } from 'lodash';
import classnames from 'classnames';

class DropDown extends React.Component {

  onClick = event => {
    event.preventDefault();
    let {options} = this.props;
    let index = $(event.target).parent().index();
    let option = options[index];
    let value = isObject(option) ? option.value : option;

    this.props.onChange(value);
  }

  renderOption(option, index){
    let displayValue = isObject(option) ? option.label : option;
    return (
      <li key={index}>
        <a href="#">{displayValue}</a>
      </li>
    )
  }

  getDisplayValue(value){
    let {options=[]} = this.props;
    for(let i = 0; i < options.length; i++){
      let op = options[i];
      if(isObject(op) && op.value === value){
        return op.label;
      }

      if(op === value){
        return value;
      }
    }

    return null;
  }

  render(){
    let {options, value, classRules, className='', name, size='default', align='left'} = this.props;
    let $options = options.map(this.renderOption);
    let hiddenValue = value;
    let displayValue = this.getDisplayValue(value);

    displayValue = displayValue || 'Select...';

    let valueClass = classnames('grv-dropdown-value', {
      'text-muted': !hiddenValue
    })

    let mainClass = `grv-dropdown ${className}`;

    let btnClass = classnames('btn btn-default full-width dropdown-toggle', {
      'btn-sm': size === 'sm'
    })
    
    let menuClass = classnames('dropdown-menu', {
      'pull-right': align === 'right'
    });

    let $menu = options.length > 0 ? (
      <ul onClick={this.onClick} className={menuClass}>
        {$options}
      </ul>
    ) : null;

    return (
      <div className={mainClass}>
        <div className = "dropdown" >
          <div className={btnClass}
            type="button"
            data-toggle="dropdown">
            <div className={valueClass}>
              <span style={{textOverflow: "ellipsis", overflow: "hidden"}}>{displayValue}</span>
              <span className="caret m-l-sm"></span>
            </div>
          </div>
          {$menu}          
        </div>
        <input className={classRules}
          value={hiddenValue}
          type="hidden"
          ref="input"
          name={name} />
      </div>
    )
  }
}

export default DropDown;