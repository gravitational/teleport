var React = require('react');
var reactor = require('app/reactor');
var {getters, actions} = require('app/modules/sessions');
var SessionList = require('./sessionList.jsx');
var {DateRangePicker, CalendarNav} = require('./../datePicker.jsx');
var moment = require('moment');
var {monthRange} = require('app/common/dateUtils');

var Sessions = React.createClass({

  mixins: [reactor.ReactMixin],

  getInitialState(){
    let [startDate, endDate] = monthRange(new Date());
    return {
      startDate,
      endDate
    }
  },

  getDataBindings() {
    return {
      sessionsView: getters.sessionsView
    }
  },

  setNewState(startDate, endDate){
    actions.fetchSessions(startDate, endDate);
    this.state.startDate = startDate;
    this.state.endDate = endDate;
    this.setState(this.state);
  },

  componentWillMount(){
    actions.fetchSessions(this.state.startDate, this.state.endDate);
  },

  componentWillUnmount: function() {
  },

  onRangePickerChange({startDate, endDate}){
    this.setNewState(startDate, endDate);
  },

  onCalendarNavChange(newValue){
    let [startDate, endDate] = monthRange(newValue);
    this.setNewState(startDate, endDate);
  },

  render: function() {
    let {startDate, endDate} = this.state;
    let data = this.state.sessionsView.filter(
      item => moment(item.created).isBetween(startDate, endDate));

    return (
      <div className="grv-sessions">
        <div className="grv-flex">
          <div className="grv-flex-column">
            <h1> Sessions</h1>
          </div>
        </div>

        <div className="grv-flex">
          <div className="grv-flex-column">
            <DateRangePicker startDate={startDate} endDate={endDate} onChange={this.onRangePickerChange}/>
          </div>
          <div className="grv-flex-column">
            <CalendarNav value={startDate} onValueChange={this.onCalendarNavChange}/>
          </div>
          <div className="grv-flex-column">
          </div>
        </div>
        <SessionList sessionRecords={data}/>
      </div>
    );
  }
});


module.exports = Sessions;
