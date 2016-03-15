var React = require('react');
var { actions} = require('app/modules/sessions');
var LinkedStateMixin = require('react-addons-linked-state-mixin');
var {Table, Column, Cell, TextCell, SortHeaderCell, SortTypes} = require('app/components/table.jsx');
var {ButtonCell, SingleUserCell, DateCreatedCell} = require('./listItems');
var {DateRangePicker, CalendarNav} = require('./../datePicker.jsx');
var moment =  require('moment');
var {monthRange} = require('app/common/dateUtils');
var {isMatch} = require('app/common/objectUtils');
var _ = require('_');

var ArchivedSessions = React.createClass({

  mixins: [LinkedStateMixin],

  getInitialState(){
    let [startDate, endDate] = monthRange(new Date());
    this.searchableProps = ['serverIp', 'created', 'sid', 'login'];
    return { filter: '', colSortDirs: {created: 'ASC'}, startDate, endDate };
  },

  componentWillMount(){
    actions.fetchSessions(this.state.startDate, this.state.endDate);
  },

  setDatesAndRefetch(startDate, endDate){
    actions.fetchSessions(startDate, endDate);
    this.state.startDate = startDate;
    this.state.endDate = endDate;
    this.setState(this.state);
  },

  onSortChange(columnKey, sortDir) {
    this.setState({
      ...this.state,
      colSortDirs: { [columnKey]: sortDir }
    });
  },

  onRangePickerChange({startDate, endDate}){
    this.setDatesAndRefetch(startDate, endDate);
  },

  onCalendarNavChange(newValue){
    let [startDate, endDate] = monthRange(newValue);
    this.setDatesAndRefetch(startDate, endDate);
  },

  searchAndFilterCb(targetValue, searchValue, propName){
    if(propName === 'created'){
      var displayDate = moment(targetValue).format('l LTS').toLocaleUpperCase();
      return displayDate.indexOf(searchValue) !== -1;
    }
  },

  sortAndFilter(data){
    var filtered = data.filter(obj=>
      isMatch(obj, this.state.filter, {
        searchableProps: this.searchableProps,
        cb: this.searchAndFilterCb
      }));

    var columnKey = Object.getOwnPropertyNames(this.state.colSortDirs)[0];
    var sortDir = this.state.colSortDirs[columnKey];
    var sorted = _.sortBy(filtered, columnKey);
    if(sortDir === SortTypes.ASC){
      sorted = sorted.reverse();
    }

    return sorted;
  },

  render: function() {
    let {startDate, endDate} = this.state;
    let data = this.props.data.filter(item => !item.active && moment(item.created).isBetween(startDate, endDate));
    data = this.sortAndFilter(data);

    return (
      <div className="grv-sessions-stored">
        <div className="grv-header">
          <h1> Archived Sessions </h1>
          <div className="grv-flex">
            <div className="grv-flex-row">
              <DateRangePicker startDate={startDate} endDate={endDate} onChange={this.onRangePickerChange}/>
            </div>
            <div className="grv-flex-row">
              <CalendarNav value={startDate} onValueChange={this.onCalendarNavChange}/>
            </div>
            <div className="grv-flex-row">
              <div className="grv-search">
                <input valueLink={this.linkState('filter')} placeholder="Search..." className="form-control input-sm"/>
              </div>
            </div>
          </div>
        </div>
        <div className="grv-content">
          <div className="">
            <Table rowCount={data.length} className="table-striped">
              <Column
                columnKey="sid"
                header={<Cell> Session ID </Cell> }
                cell={<TextCell data={data}/> }
              />
              <Column
                header={<Cell> </Cell> }
                cell={
                  <ButtonCell data={data} />
                }
              />
              <Column
                columnKey="created"
                header={
                  <SortHeaderCell
                    sortDir={this.state.colSortDirs.created}
                    onSortChange={this.onSortChange}
                    title="Created"
                  />
                }
                cell={<DateCreatedCell data={data}/> }
              />
              <Column
                header={<Cell> User </Cell> }
                cell={<SingleUserCell data={data}/> }
              />
            </Table>
          </div>
        </div>
      </div>
    )
  }
});

module.exports = ArchivedSessions;
