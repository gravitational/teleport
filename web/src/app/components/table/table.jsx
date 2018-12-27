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

const TableTextCell = ({rowIndex, data, columnKey, ...props}) => (
  <TableCell {...props}>
    {data[rowIndex][columnKey]}
  </TableCell>
);

/**
* Sort indicator used by SortHeaderCell
*/
const SortTypes = {
  ASC: 'ASC',
  DESC: 'DESC'
};

const SortIndicator = ({sortDir})=>{
  let cls = 'grv-table-indicator-sort fa fa-sort'
  if(sortDir === SortTypes.DESC){
    cls += '-desc'
  }

  if( sortDir === SortTypes.ASC){
    cls += '-asc'
  }

  return (<i className={cls}></i>);
};

/**
* Sort Header Cell
*/
class SortHeaderCell extends React.Component {

  onSortChange = e => {
    e.preventDefault();
    if(this.props.onSortChange) {
      // default
      let newDir = SortTypes.DESC;
      if(this.props.sortDir){
        newDir = this.props.sortDir === SortTypes.DESC ? SortTypes.ASC : SortTypes.DESC;
      }
      this.props.onSortChange(this.props.columnKey, newDir);
    }
  }

  render() {
    const { sortDir, title, ...props } = this.props;

    return (
      <TableCell {...props}>
        <a onClick={this.onSortChange}>
          {title}
        </a>
        <SortIndicator sortDir={sortDir}/>
      </TableCell>
    );
  }
}

/**
* Default Cell
*/
const TableCell = props => {
  let { isHeader, children, className='' } = props;
  className = 'grv-table-cell ' + className;
  return isHeader ? <th className={className}>{children}</th> : <td>{children}</td>;
}


/**
* Table
*/

class Table extends React.Component {

  renderHeader(children){
    const { data } = this.props;
    const cells = children.map((item, index) => {
      return this.renderCell(
        item.props.header,
        {
          index,
          key: index,
          isHeader: true,
          data,
          ...item.props
        });
    });

    return (
      <thead className="grv-table-header">
        <tr>{cells}</tr>
      </thead>
    )
  }

  renderBody(children) {
    const { data } = this.props;
    const count = this.props.rowCount;
    const rows = [];
    for (var i = 0; i < count; i++){
      var cells = children.map((item, index)=>{
        return this.renderCell(
          item.props.cell,
          {
            rowIndex: i,
            key: index,
            isHeader: false,
            data,
            ...item.props
          }
        );
      })

      rows.push(<tr key={i}>{cells}</tr>);
    }

    return (
      <tbody>{rows}</tbody>
    );
  }

  renderCell(cell, cellProps){
    var content = null;
    if (React.isValidElement(cell)) {
       content = React.cloneElement(cell, cellProps);
     } else if (typeof cell === 'function') {
       content = cell(cellProps);
     }

     return content;
  }

  render() {
    var children = [];
    React.Children.forEach(this.props.children, (child) => {
      if (child == null) {
        return;
      }

      if(!child.props._isColumn){
        throw 'Should be Column';
      }

      children.push(child);
    });

    var tableClass = 'table grv-table ' + this.props.className;

    return (
      <table className={tableClass}>
        {this.renderHeader(children)}
        {this.renderBody(children)}
      </table>
    );
  }
}

class Column extends React.Component {
  static defaultProps = {
    _isColumn: true
  }

  render(){
    throw new Error('Component <Column /> should never render');
  }
}

const EmptyIndicator = ({text}) => (
  <div className="grv-table-indicator-empty text-muted"><span>{text}</span></div>
)

export default Table;
export {
  Column,
  Table,
  TableCell as Cell,
  TableTextCell as TextCell,
  SortHeaderCell,
  SortIndicator,
  SortTypes,
  EmptyIndicator};