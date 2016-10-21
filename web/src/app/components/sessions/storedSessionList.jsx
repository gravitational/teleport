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

var _ = require('_');
var React = require('react');
var moment =  require('moment');
var InputSearch = require('./../inputSearch.jsx');
var { isMatch } = require('app/common/objectUtils');
var { displayDateFormat} = require('app/config');
var { actions } = require('app/modules/storedSessionsFilter');
var { Table, Column, Cell, TextCell, SortHeaderCell, SortTypes, EmptyIndicator } = require('app/components/table.jsx');
var { ButtonCell, SingleUserCell, DateCreatedCell } = require('./listItems');
var { DateRangePicker } = require('./../datePicker.jsx');


var ArchivedSessions = React.createClass({

  getInitialState(){
    this.searchableProps = ['clientIp', 'nodeIp', 'created', 'sid', 'login'];
    return { filter: '', colSortDirs: {created: 'ASC'}};
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
    /**
    * as date picker uses timeouts its important to ensure that
    * component is still mounted when data picker triggers an update
    */
    if(this.isMounted()){
      actions.setTimeRange(startDate, endDate);
    }
  },

  searchAndFilterCb(targetValue, searchValue, propName){
    if(propName === 'created'){
      var displayDate = moment(targetValue).format(displayDateFormat).toLocaleUpperCase();
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

  render() {
    let { start, end } = this.props.filter;
    let data = this.props.data.filter(
      item => !item.active && moment(item.created).isBetween(start, end));

    data = this.sortAndFilter(data);

    return (
      <div className="grv-sessions-stored">
        <div className="grv-header">
          <div className="grv-flex m-b-md">
            <div className="grv-flex-column"></div>
            <div className="grv-flex-column">
              <h2 className="text-center"> Archived Sessions </h2>
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
          {data.length === 0 ? <EmptyIndicator text="No matching archived sessions found."/> :
            <div className="">
              <Table rowCount={data.length} className="table-striped">
                <Column
                  header={<Cell/>}
                  cell={
                    <ButtonCell data={data} />
                  }
                />
                <Column
                  columnKey="nodeIp"
                  header={<Cell> Node IP </Cell> }
                  cell={<TextCell data={data} /> }
                />
                <Column
                  columnKey="clientIp"
                  header={<Cell> Client IP </Cell> }
                  cell={<TextCell data={data} /> }
                />
                <Column
                  columnKey="sid"
                  header={<Cell> Session ID </Cell> }
                  cell={<TextCell data={data}/> }
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
      </div>
    )
  }
});

module.exports = ArchivedSessions;
