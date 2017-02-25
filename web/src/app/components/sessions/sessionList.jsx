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

import _ from '_';
import React from 'react';
import moment from 'moment';
import InputSearch from './../inputSearch.jsx';
import { isMatch } from 'app/common/objectUtils';
import { actions } from 'app/modules/storedSessionsFilter';
import { Table, Column, Cell, SortHeaderCell, SortTypes, EmptyIndicator } from 'app/components/table.jsx';
import { SessionIdCell, NodeCell, UsersCell, DateCreatedCell, DurationCell } from './listItems';
import { DateRangePicker } from './../datePicker.jsx';
import ClusterSelector from './../clusterSelector.jsx';

const SessionList = React.createClass({

  getInitialState(){
    this.searchableProps = ['nodeDisplayText', 'createdDisplayText', 'sid', 'parties'];
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
    if (propName === 'parties') {
      targetValue = targetValue || [];
      return targetValue.join('').toLocaleUpperCase().indexOf(searchValue) !== -1;
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
    let { filter, storedSessions, activeSessions } = this.props;
    let { start, end } = filter;
    
    let stored = storedSessions.filter(
      item => moment(item.created).isBetween(start, end));

    let active = activeSessions.filter(
      item => moment(item.created).isBetween(start, end));    

    stored = this.sortAndFilter(stored);
    active = this.sortAndFilter(active);

    // always display active sessions first    
    let data = [...active, ...stored];
  
    return (
      <div className="grv-sessions-stored m-t">
        <div className="grv-header">
          <div className="grv-flex m-b-md" style={{ justifyContent: "space-between" }}>
            <div className="grv-flex">  
              <h2 className="text-center"> Sessions </h2>            
            </div>            
            <div className="grv-flex">              
              <ClusterSelector/>
              <InputSearch value={this.filter} onChange={this.onFilterChange} />
              <div className="m-l-sm">
                <DateRangePicker startDate={start} endDate={end} onChange={this.onRangePickerChange} />
              </div>
            </div>
          </div>                  
        </div>
        <div className="grv-content">
          {data.length === 0 ? <EmptyIndicator text="No matching sessions found"/> :            
            <Table rowCount={data.length}>
              <Column
                header={<Cell className="grv-sessions-col-sid"> Session ID </Cell> }
                cell={
                  <SessionIdCell data={data} />
                }
              />                                
              <Column
                header={<Cell> User </Cell> }
                cell={<UsersCell data={data}/> }
              />
              <Column
                columnKey="nodeIp"
                header={
                  <Cell className="grv-sessions-stored-col-ip">Node</Cell>
                }
                cell={<NodeCell data={data} /> }
              />
              <Column
                columnKey="created"
                header={
                  <SortHeaderCell
                    sortDir={this.state.colSortDirs.created}
                    onSortChange={this.onSortChange}
                    title="Created (UTC)"
                  />
                }
                cell={<DateCreatedCell data={data}/> }
              />
              <Column
                columnKey="duration"
                header={
                  <SortHeaderCell
                    sortDir={this.state.colSortDirs.duration}
                    onSortChange={this.onSortChange}
                    title="Duration"
                  />
                }
                cell={<DurationCell data={data} /> }
              />                
            </Table>            
          }
        </div>
      </div>
    )
  }
});

export default SessionList;
