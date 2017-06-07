import React from 'react';
import ReactSelect from 'react-select';
import $ from 'jQuery';
import {isObject} from 'lodash';
import classnames from 'classnames';

var InputDropDown = React.createClass({

  onClick(event){
    let {options} = this.props;
    let index = $(event.target).parent().index();
    let option = options[index];
    let value = isObject(option) ? option.value : option;
    this.props.onChange(value);
  },

  renderOption(option, index){
    let displayValue = isObject(option) ? option.label : option;
    return (
      <li key={index}>
        <a href="#">{displayValue}</a>
      </li>
    )
  },

  onValueChange(event){
    if( this.props.onChange){
      this.props.onChange(event.target.value);
    }
  },

  render(){
    let {options, value, classRules, name} = this.props;
    let $options = options.map(this.renderOption);

    return (
      <div className="grv-dropdown">
        <div className = "input-group" >
          {
            options.length > 0 ?
              <div className="input-group-btn">
                <button type="button" className="btn btn-default dropdown-toggle" data-toggle="dropdown" aria-haspopup="trufe" aria-expanded="false">
                  <span className="caret"></span>
                </button>
                <ul onClick={this.onClick} className="dropdown-menu dropdown-menu-right">
                  {$options}
                </ul>
              </div> : null
          }
          <input onChange={this.onValueChange} type="text" value={value} className="form-control" aria-label="Text input with segmented button dropdown"/>
        </div>
        <input className={classRules} value={value} type="hidden" ref="input" name={name}/>
      </div>
    )
  }
});

var DropDown = React.createClass({

  onClick(event){
    event.preventDefault();
    let {options} = this.props;
    let index = $(event.target).parent().index();
    let option = options[index];
    let value = isObject(option) ? option.value : option;

    this.props.onChange(value);
  },

  renderOption(option, index){
    let displayValue = isObject(option) ? option.label : option;
    return (
      <li key={index}>
        <a href="#">{displayValue}</a>
      </li>
    )
  },

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
  },

  render(){
    let {options, value, classRules, name, size='default'} = this.props;
    let $options = options.map(this.renderOption);
    let hiddenValue = value;

    let displayValue = this.getDisplayValue(value);

    displayValue = displayValue || 'Select...';

    let valueClass = classnames('grv-dropdown-value', {
      'text-muted': !hiddenValue
    })

    let btnClass = classnames('btn btn-default full-width dropdown-toggle', {
      'btn-sm': size === 'sm'
    })

    return (
      <div className="grv-dropdown">
        <div className = "dropdown" >
          <div className={btnClass} type="button" id="dropdownMenu1" data-toggle="dropdown" aria-haspopup="true" aria-expanded="true">
            <div className={valueClass}>
              <span style={{textOverflow: "ellipsis", overflow: "hidden"}}>{displayValue}</span>
              <span className="caret m-l-sm"></span>
            </div>
          </div>
          {
            options.length > 0 ?
                <ul onClick={this.onClick} className="dropdown-menu dropdown-menu-right">
                  {$options}
                </ul>
               : null
          }
        </div>
        <input className={classRules} value={hiddenValue} type="hidden" ref="input" name={name}/>
      </div>
    )
  }
});

var Select = React.createClass({

  onChange(value){
    $(this.refs.input).val(value);
    if(this.props.onChange){
      this.props.onChange(value);
    }
  },

  componentDidMount(){
    this.ensureValidationPlaceholder();
  },

  ensureValidationPlaceholder(){
    let { value } = this.props;
    $(this.refs.input).val(value);
  },

  componentDidUpdate(){
    this.ensureValidationPlaceholder();
  },

  render(){
    let props = this.props;
    let {classRules='', name, options} = this.props;

    // if just an array of strings, convert it to { value, label } format
    if(Array.isArray(options) && !isObject(options[0])){
      options = options.map( item => ({ value: item, label: item}));
    }

    let childProps = {
      searchable: false,
      clearable: false,
      ...props,
      options,
       onChange: this.onChange
    }

    return (
      <div>
        <ReactSelect {...childProps} ref="container"/>
        <input className={classRules} type="hidden" ref="input" name={name} />
    </div>
    )
  }
});

export {
  Select,
  InputDropDown,
  DropDown
}
