/*
Copyright 2019 Gravitational, Inc.

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
import { sortBy } from 'lodash';
import isMatch from 'design/utils/match';
import { TablePaged, Column, SortHeaderCell, Cell, TextCell, SortTypes } from 'design/DataTable';
import EventTypeCell from './EventTypeCell';
import EventDescCell from './EventDescCell';
import { ActionCell, TimeCell } from './EventListCells';
import EventDialog from './../EventDialog';

class EventList extends React.Component {

  searchableProps = ['codeDesc', 'message', 'user', 'time' ]

  constructor(props) {
    super(props);
    this.state = {
      detailsToShow: null,
      colSortDirs: {
        time: SortTypes.ASC
      }
    }
  }

  onSortChange = (columnKey, sortDir) => {
    this.state.colSortDirs = { [columnKey]: sortDir };
    this.setState(this.state);
  }

  sortAndFilter(data, searchValue) {
    const { colSortDirs } = this.state;
    const filtered = data
      .filter(obj => isMatch(obj, searchValue, {
        searchableProps: this.searchableProps
      }));

    const columnKey = Object.getOwnPropertyNames(colSortDirs)[0];
    const sortDir = colSortDirs[columnKey];
    const sorted = sortBy(filtered, columnKey);
    if(sortDir === SortTypes.ASC){
      return sorted.reverse();
    }

    return sorted;
  }

  showDetails = detailsToShow => {
    this.setState({
      detailsToShow
    })
  }

  closeDetails = () => {
    this.setState({
      detailsToShow: null
    })
  }

  render() {
    const { events=[], search='', pageSize=20, limit=0 } = this.props;
    const { detailsToShow } = this.state;

    let sorted = this.sortAndFilter(events, search);

    if(limit > 0){
      sorted = sorted.slice(0, limit);
    }

    return (
      <React.Fragment>
        <TablePaged data={sorted} pageSize={pageSize}>
          <Column
            columnKey="codeDesc"
            cell={<EventTypeCell /> }
            header={
              <SortHeaderCell
                sortDir={this.state.colSortDirs.codeDesc}
                onSortChange={this.onSortChange}
                title="Type"
              />}
          />
          <Column
            columnKey="message"
            header={
              <Cell>Description</Cell>
            }
            cell={<EventDescCell style={{wordBreak: "break-all"}}/> }
          />
          <Column
            columnKey="user"
            header={
              <SortHeaderCell
                sortDir={this.state.colSortDirs.user}
                onSortChange={this.onSortChange}
                title="User"
              />
            }
            cell={<TextCell /> }
          />
          <Column
            columnKey="time"
            header={
              <SortHeaderCell
                sortDir={this.state.colSortDirs.time}
                onSortChange={this.onSortChange}
                title="Created"
              />
            }
            cell={<TimeCell /> }
          />
          <Column
            header={<Cell/>}
            cell={ <ActionCell onViewDetails={this.showDetails} /> }
          />
        </TablePaged>
        { detailsToShow && (
          <EventDialog event={detailsToShow}
            onClose={this.closeDetails}  />
          )
        }
      </React.Fragment>
    )
  }
}

export default EventList;
