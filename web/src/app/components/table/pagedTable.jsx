/*
Copyright 2018 Gravitational, Inc.

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
import classnames from 'classnames';
import { Table } from './table.jsx';

const PagedTable = React.createClass({

  onPrev(){
    let { startFrom, pageSize } = this.state;

    startFrom = startFrom - pageSize;

    if( startFrom < 0){
      startFrom = 0;
    }

    this.setState({
      startFrom
    })

  },

  onNext(){
    const { data=[] } = this.props;
    const { startFrom, pageSize } = this.state;
    let newStartFrom = startFrom + pageSize;

    if( newStartFrom < data.length){
      newStartFrom = startFrom + pageSize;
      this.setState({
        startFrom: newStartFrom
      })
    }
  },

  getInitialState(){
    const { pageSize=7 } = this.props;
    return {
      startFrom: 0,
      pageSize
    }
  },

  componentWillReceiveProps(newProps){
    const newData = newProps.data || [];
    const oldData = this.props.data || [];
    // if data length changes, reset paging
    if(newData.length !== oldData.length){
      this.setState({startFrom: 0})
    }
  },

  render(){
    const { startFrom, pageSize } = this.state;
    const { data=[], tableClass='', className='' } = this.props;
    const totalRows = data.length;

    let endAt = 0;
    let pagedData = data;

    if (data.length > 0){
      endAt = startFrom + (pageSize > data.length ? data.length : pageSize);

      if(endAt > data.length){
        endAt = data.length;
      }

      pagedData = data.slice(startFrom, endAt);
    }

    const tableProps = {
      ...this.props,
      rowCount: pagedData.length,
      data: pagedData
    }

    const infoProps = {
      pageSize,
      startFrom,
      endAt,
      totalRows
    }

    return (
      <div className={className}>
        <PageInfo {...infoProps} onPrev={this.onPrev} onNext={this.onNext} />
        <div className={tableClass}>
          <Table {...tableProps} />
        </div>
        <PageInfo {...infoProps} onPrev={this.onPrev} onNext={this.onNext} />
      </div>
    )
  }
});

const PageInfo = props => {
  const {startFrom, endAt, totalRows, onPrev, onNext, pageSize} = props;

  const shouldBeDisplayed = totalRows > pageSize;

  if(!shouldBeDisplayed){
    return null;
  }

  const prevBtnClass = classnames('btn btn-white', {
    'disabled': startFrom === 0
  });

  const nextBtnClass = classnames('btn btn-white', {
    'disabled': endAt === totalRows
  });

  return (
    <div className="m-b-sm grv-table-paged-info">
      <span className="m-r-sm">
        <span className="text-muted">Showing </span>
        <span className="font-bold">{startFrom+1}</span>
        <span className="text-muted"> to </span>
        <span className="font-bold">{endAt}</span>
        <span className="text-muted"> of </span>
        <span className="font-bold">{totalRows}</span>
      </span>
      <div className="btn-group btn-group-sm">
        <a onClick={onPrev} className={prevBtnClass} type="button">Prev</a>
        <a onClick={onNext} className={nextBtnClass} type="button">Next</a>
      </div>
    </div>
  )
}

export default PagedTable;

export {
  PagedTable
};
