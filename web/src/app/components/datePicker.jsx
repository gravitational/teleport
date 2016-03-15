var React = require('react');
var $ = require('jQuery');
var moment = require('moment');
var {debounce} = require('_');

var DateRangePicker = React.createClass({

  getDates(){
    var startDate = $(this.refs.dpPicker1).datepicker('getDate');
    var endDate = $(this.refs.dpPicker2).datepicker('getDate');
    return [startDate, endDate];
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
      autoclose: true
    }).on('changeDate', this.onChange);

    this.setDates(this.props);
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
      <div className="grv-datepicker input-group input-daterange" ref="rangePicker">        
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

/**
* Calendar Nav
*/
var CalendarNav = React.createClass({

  render() {
    let {value} = this.props;
    let displayValue = moment(value).format('MMMM, YYYY');

    return (
      <div className={"grv-calendar-nav " + this.props.className} >
        <button onClick={this.move.bind(this, -1)} className="btn btn-outline btn-link"><i className="fa fa-chevron-left"></i></button>
        <span className="text-muted">{displayValue}</span>
        <button onClick={this.move.bind(this, 1)} className="btn btn-outline btn-link"><i className="fa fa-chevron-right"></i></button>
      </div>
    );
  },

  move(at){
    let {value} = this.props;
    let newValue = moment(value).add(at, 'month').toDate();
    this.props.onValueChange(newValue);
  }
});

CalendarNav.getMonthRange = function(value){
  let startDate = moment(value).startOf('month').toDate();
  let endDate = moment(value).endOf('month').toDate();
  return [startDate, endDate];
}

export default DateRangePicker;
export {CalendarNav, DateRangePicker};
