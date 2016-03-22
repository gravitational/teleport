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
var InputSearch = require('./../inputSearch.jsx');
var {Table, Column, Cell, SortHeaderCell, SortTypes, EmptyIndicator} = require('app/components/table.jsx');
var {createNewSession} = require('app/modules/currentSession/actions');

var _ = require('_');
var {isMatch} = require('app/common/objectUtils');

const TextCell = ({rowIndex, data, columnKey, ...props}) => (
  <Cell {...props}>
    {data[rowIndex][columnKey]}
  </Cell>
);

const TagCell = ({rowIndex, data, ...props}) => (
  <Cell {...props}>
    { data[rowIndex].tags.map((item, index) =>
      (<span key={index} className="label label-default">
        {item.role} <li className="fa fa-long-arrow-right"></li>
        {item.value}
      </span>)
    ) }
  </Cell>
);

const LoginCell = ({logins, onLoginClick, rowIndex, data, ...props}) => {
  if(!logins ||logins.length === 0){
    return <Cell {...props} />;
  }

  var serverId = data[rowIndex].id;
  var $lis = [];

  function onClick(i){
    var login = logins[i];
    if(onLoginClick){
      return ()=> onLoginClick(serverId, login);
    }else{
      return () => createNewSession(serverId, login);
    }
  }

  for(var i = 0; i < logins.length; i++){
    $lis.push(<li key={i}><a onClick={onClick(i)}>{logins[i]}</a></li>);
  }

  return (
    <Cell {...props}>
      <div className="btn-group">
        <button type="button" onClick={onClick(0)} className="btn btn-xs btn-primary">{logins[0]}</button>
        {
          $lis.length > 1 ? (
              [
                <button key={0} data-toggle="dropdown" className="btn btn-default btn-xs dropdown-toggle" aria-expanded="true">
                  <span className="caret"></span>
                </button>,
                <ul key={1} className="dropdown-menu">
                  {$lis}
                </ul>
              ] )
            : null
        }
      </div>
    </Cell>
  )
};

var NodeList = React.createClass({

  getInitialState(/*props*/){
    this.searchableProps = ['addr', 'hostname', 'tags'];
    return { filter: '', colSortDirs: {hostname: SortTypes.DESC} };
  },

  onSortChange(columnKey, sortDir) {
    this.state.colSortDirs = { [columnKey]: sortDir };
    this.setState(this.state);
  },

  onFilterChange(value){
    this.state.filter = value;
    this.setState(this.state);
  },

  searchAndFilterCb(targetValue, searchValue, propName){
    if(propName === 'tags'){
      return targetValue.some((item) => {
        let {role, value} = item;
        return role.toLocaleUpperCase().indexOf(searchValue) !==-1 ||
          value.toLocaleUpperCase().indexOf(searchValue) !==-1;
      });
    }
  },

  sortAndFilter(data){
    var filtered = data.filter(obj=> isMatch(obj, this.state.filter, {
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
    var data = this.sortAndFilter(this.props.nodeRecords);
    var logins = this.props.logins;
    var onLoginClick = this.props.onLoginClick;

    return (
      <div className="grv-nodes grv-page">
        <div className="grv-flex grv-header">
          <div className="grv-flex-column"></div>
          <div className="grv-flex-column">
            <h1> Nodes </h1>
          </div>
          <div className="grv-flex-column">
            <InputSearch value={this.filter} onChange={this.onFilterChange}/>
          </div>
        </div>
        <div className="">
          {
            data.length === 0 && this.state.filter.length > 0 ? <EmptyIndicator text="No matching nodes found."/> :

            <Table rowCount={data.length} className="table-striped grv-nodes-table">
              <Column
                columnKey="hostname"
                header={
                  <SortHeaderCell
                    sortDir={this.state.colSortDirs.hostname}
                    onSortChange={this.onSortChange}
                    title="Node"
                  />
                }
                cell={<TextCell data={data}/> }
              />
              <Column
                columnKey="addr"
                header={
                  <SortHeaderCell
                    sortDir={this.state.colSortDirs.addr}
                    onSortChange={this.onSortChange}
                    title="IP"
                  />
                }

                cell={<TextCell data={data}/> }
              />
              <Column
                columnKey="tags"
                header={<Cell></Cell> }
                cell={<TagCell data={data}/> }
              />
              <Column
                columnKey="roles"
                onLoginClick={onLoginClick}
                header={<Cell>Login as</Cell> }
                cell={<LoginCell data={data} logins={logins}/> }
              />
            </Table>
          }
        </div>
      </div>
    )
  }
});

module.exports = NodeList;
