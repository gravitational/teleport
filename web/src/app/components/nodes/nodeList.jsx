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
var DropDown = require('./../dropdown.jsx');

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

  var { id, siteId } = data[rowIndex];
  var $lis = [];

  function onClick(i){
    var login = logins[i];
    if(onLoginClick){
      return () => onLoginClick(id, login);
    }else{
      return () => createNewSession(siteId, id, login);
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

const ALL_CLUSTERS = ' ---all--- ';
const OptionShowAllSites = { value: ALL_CLUSTERS, label: 'all clusters' };

var NodeList = React.createClass({

  getInitialState() {                            
    this.searchableProps = ['addr', 'hostname', 'tags'];
    return {
        selectedSite: ALL_CLUSTERS,
        filter: '',
        colSortDirs: { hostname: SortTypes.DESC }
    };
  },

  onSortChange(columnKey, sortDir) {
    this.state.colSortDirs = { [columnKey]: sortDir };
    this.setState(this.state);
  },

  onFilterChange(value){
    this.state.filter = value;
    this.setState(this.state);
  },

  onChangeSite(value) {  
    this.setState({
      selectedSite: value  
    })      
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

  sortAndFilter(data) {
    let { selectedSite, colSortDirs } = this.state;    
    let filtered = data      
      .filter(obj=> isMatch(obj, this.state.filter, {
        searchableProps: this.searchableProps,
        cb: this.searchAndFilterCb
      }));
    
    if (selectedSite !== ALL_CLUSTERS) {
      filtered = filtered.filter(obj => obj.siteId === selectedSite)
    }

    let columnKey = Object.getOwnPropertyNames(colSortDirs)[0];
    let sortDir = colSortDirs[columnKey];
    let sorted = _.sortBy(filtered, columnKey);
    if(sortDir === SortTypes.ASC){
      sorted = sorted.reverse();
    }

    return sorted;
  },

  render() {  
    let { nodeRecords, logins, onLoginClick, sites } = this.props;
    let { selectedSite } = this.state;      
    let data = this.sortAndFilter(nodeRecords, selectedSite);            
    let siteOptions = sites.map(s => ({ label: s.name, value: s.name }));
    
    siteOptions.push(OptionShowAllSites);
        
    return (
      <div className="grv-nodes grv-page">
        <div className="grv-flex grv-header m-t-md" style={{ justifyContent: "space-between" }}>                    
          <h2 className="text-center no-margins"> Nodes </h2>          
          <div className="grv-flex">          
            <DropDown
              className="grv-nodes-clusters-selector m-r"
              size="sm"              
              onChange={this.onChangeSite}
              value={selectedSite}
              options={siteOptions}
            />
            <InputSearch value={this.filter} onChange={this.onFilterChange} />                        
          </div>
        </div>
        <div className="m-t">
          {
            data.length === 0 && this.state.filter.length > 0 ? <EmptyIndicator text="No matching nodes found"/> :

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
                header={<Cell /> }
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

export default NodeList;
