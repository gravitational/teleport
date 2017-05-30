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
import moment from 'moment';
import {debounce} from 'lodash';

const DateRangePicker = React.createClass({

  getDates(){
    var startDate = $(this.refs.dpPicker1).datepicker('getDate');
    var endDate = $(this.refs.dpPicker2).datepicker('getDate');
    return [startDate, moment(endDate).endOf('day').toDate()];
  },

  setDates({startDate, endDate}){
    $(this.refs.dpPicker1).datepicker('setDate', startDate);
    $(this.refs.dpPicker2).datepicker('setDate', endDate);
  },

  getDefaultProps() {
     return {
       startDate: moment().startOf('month').toDate(),
       endDate: moment().endOf('month').toDate(),
       onChange: ()=>{}
     };
   },

  componentWillUnmount(){
    $(this.refs.dp).datepicker('destroy');
  },

  componentWillReceiveProps(newProps){
    var [startDate, endDate] = this.getDates();
    if(!(isSame(startDate, newProps.startDate) &&
          isSame(endDate, newProps.endDate))){
        this.setDates(newProps);
      }
  },

  shouldComponentUpdate(){
    return false;
  },

  componentDidMount(){        
    this.onChange = debounce(this.onChange, 1);
    $(this.refs.rangePicker).datepicker({
      todayBtn: 'linked',
      keyboardNavigation: false,
      forceParse: false,
      calendarWeeks: true,
      autoclose: true,      
    });
        
    this.setDates(this.props);

    $(this.refs.rangePicker).datepicker()
      .on('changeDate', this.onChange);    
  },

  onChange(){    
    var [startDate, endDate] = this.getDates()
    if(!(isSame(startDate, this.props.startDate) &&
          isSame(endDate, this.props.endDate))){
        this.props.onChange({startDate, endDate});
    }
  },

  render() {
    return (
      <div className="grv-datepicker input-group input-group-sm input-daterange" ref="rangePicker">
        <input ref="dpPicker1" type="text" className="input-sm form-control" name="start" />
        <span className="input-group-addon">to</span>
        <input ref="dpPicker2" type="text" className="input-sm form-control" name="end" />
      </div>
    );
  }
});

function isSame(date1, date2){
  return moment(date1).isSame(date2, 'day');
}

export default DateRangePicker;