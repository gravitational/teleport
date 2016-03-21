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

var React = require('react');
var {actions} = require('app/modules/storedSessionsFilter');
var InputSearch = require('./../inputSearch.jsx');
var {Table, Column, Cell, TextCell, SortHeaderCell, SortTypes} = require('app/components/table.jsx');
var {ButtonCell, SingleUserCell, EmptyList, DateCreatedCell} = require('./listItems');
var {DateRangePicker} = require('./../datePicker.jsx');
var moment =  require('moment');
var {weekRange} = require('app/common/dateUtils');
var {isMatch} = require('app/common/objectUtils');
var _ = require('_');

var ArchivedSessions = React.createClass({

  getInitialState(){
    this.searchableProps = ['serverIp', 'created', 'sid', 'login'];
    return { filter: '', colSortDirs: {created: 'ASC'}};
  },

  componentWillMount(){
    setTimeout(()=>actions.fetch(), 0);
  },

  componentWillUnmount(){
    actions.removeStoredSessions();
  },

  onFilterChange(value){
    this.state.filter = value;
    this.setState(this.state);
  },

  onSortChange(columnKey, sortDir) {
    this.state.colSortDirs = { [columnKey]: sortDir };
    this.setState(this.state);
  },

  onRangePickerChange({startDate, endDate}){
    actions.setTimeRange(startDate, endDate);
  },

  onCalendarNavChange(newValue){
    let [startDate, endDate] = weekRange(newValue);
    actions.setTimeRange(startDate, endDate);
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
    let {start, end, status} = this.props.filter;
    let data = this.props.data.filter(
      item => !item.active && moment(item.created).isBetween(start, end));

    data = this.sortAndFilter(data);

    return (
      <div className="grv-sessions-stored">
        <div className="grv-header">
          <div className="grv-flex">
            <div className="grv-flex-column"></div>
            <div className="grv-flex-column">
              <h1> Archived Sessions </h1>
            </div>
            <div className="grv-flex-column">
              <InputSearch value={this.filter} onChange={this.onFilterChange}/>
            </div>
          </div>
          <div className="grv-flex">
            <div className="grv-flex-row">
            </div>
            <div className="grv-flex-row">
              <DateRangePicker startDate={start} endDate={end} onChange={this.onRangePickerChange}/>
            </div>
            <div className="grv-flex-row">
          </div>
        </div>
        </div>

        <div className="grv-content">
          {data.length === 0 && !status.isLoading ? <EmptyList text="No matching archived sessions found."/> :
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
          }
        </div>
        {
          status.hasMore ?
            (<div className="grv-footer">
              <button disabled={status.isLoading} className="btn btn-primary btn-outline" onClick={actions.fetchMore}>
                <span>Load more...</span>
              </button>
            </div>) : null
        }
      </div>
    )
  }
});

module.exports = ArchivedSessions;
