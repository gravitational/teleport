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

const GrvTableTextCell = ({rowIndex, data, columnKey, ...props}) => (
  <GrvTableCell {...props}>
    {data[rowIndex][columnKey]}
  </GrvTableCell>
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
var SortHeaderCell = React.createClass({
  render() {
    var {sortDir, title, ...props} = this.props;

    return (
      <GrvTableCell {...props}>
        <a onClick={this.onSortChange}>
          {title}
        </a>
        <SortIndicator sortDir={sortDir}/>
      </GrvTableCell>
    );
  },

  onSortChange(e) {
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
});

/**
* Default Cell
*/
var GrvTableCell = React.createClass({
  render(){
    var props = this.props;
    return props.isHeader ? <th key={props.key} className="grv-table-cell">{props.children}</th> : <td key={props.key}>{props.children}</td>;
  }
});

/**
* Table
*/
var GrvTable = React.createClass({

  renderHeader(children){
    var cells = children.map((item, index)=>{
      return this.renderCell(item.props.header, {index, key: index, isHeader: true, ...item.props});
    })

    return <thead className="grv-table-header"><tr>{cells}</tr></thead>
  },

  renderBody(children){
    var count = this.props.rowCount;
    var rows = [];
    for(var i = 0; i < count; i ++){
      var cells = children.map((item, index)=>{
        return this.renderCell(item.props.cell, {rowIndex: i, key: index, isHeader: false, ...item.props});
      })

      rows.push(<tr key={i}>{cells}</tr>);
    }

    return <tbody>{rows}</tbody>;
  },

  renderCell(cell, cellProps){
    var content = null;
    if (React.isValidElement(cell)) {
       content = React.cloneElement(cell, cellProps);
     } else if (typeof cell === 'function') {
       content = cell(cellProps);
     }

     return content;
  },

  render() {
    var children = [];
    React.Children.forEach(this.props.children, (child) => {
      if (child == null) {
        return;
      }

      if(child.type.displayName !== 'GrvTableColumn'){
        throw 'Should be GrvTableColumn';
      }

      children.push(child);
    });

    var tableClass = 'table ' + this.props.className;

    return (
      <table className={tableClass}>
        {this.renderHeader(children)}
        {this.renderBody(children)}
      </table>
    );
  }
})

var GrvTableColumn = React.createClass({
  render: function() {
    throw new Error('Component <GrvTableColumn /> should never render');
  }
})

const EmptyIndicator = ({text}) => (
  <div className="grv-table-indicator-empty text-center text-muted"><span>{text}</span></div>
)

export default GrvTable;
export {
  GrvTableColumn as Column,
  GrvTable as Table,
  GrvTableCell as Cell,
  GrvTableTextCell as TextCell,
  SortHeaderCell,
  SortIndicator,
  SortTypes,
  EmptyIndicator};
