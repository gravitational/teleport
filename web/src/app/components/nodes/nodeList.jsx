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
import { Link } from  'react-router';
import { sortBy } from 'lodash';
import { isMatch } from 'app/lib/objectUtils';
import InputSearch from './../inputSearch';
import { Table, Column, Cell, TextCell, SortHeaderCell, SortTypes, EmptyIndicator } from 'app/components/table.jsx';
import ClusterSelector from './../clusterSelector.jsx';
import cfg from 'app/config';
import history from 'app/services/history';

const EmptyValue = ({ text='Empty' }) => (    
  <small className="text-muted">
    <span>{text}</span>
  </small>
);
    
const TagCell = ({rowIndex, data, ...props}) => {
  const { tags } = data[rowIndex];    
  let $content = tags.map((item, index) => (
    <span key={index} title={`${item.role}:${item.value}`} className="label label-default grv-nodes-table-label">
      {item.role} <li className="fa fa-long-arrow-right m-r-xs"/> 
      {item.value}
    </span>
  ));

  if ($content.length === 0) {
    $content = <EmptyValue text="No assigned labels"/>
  }
  
  return (
    <Cell {...props}>
      {$content}
    </Cell>
  )  
}

class LoginCell extends React.Component {
 
  onKeyPress = e => {    
    if (e.key === 'Enter' && e.target.value) {        
      const url = this.makeUrl(e.target.value);
      history.push(url);
    }        
  }

  onShowLoginsClick = () => {
    this.refs.customLogin.focus()
  }

  makeUrl(login) {
    const { data, rowIndex } = this.props;
    const { siteId, id } = data[rowIndex];
    return cfg.getTerminalLoginUrl({
      siteId: siteId,
      serverId: id,
      login
    })    
  }

  render() {  
    const { logins, ...props } = this.props;                      
    const $lis = [];    
    const defaultLogin = logins[0] || 'root';
    const defaultTermUrl = this.makeUrl(defaultLogin);

    for (var i = 0; i < logins.length; i++) {
      const termUrl = this.makeUrl(logins[i]);      
      $lis.push(
        <li key={i}>
          <Link to={termUrl}>
            {logins[i]}
          </Link>
        </li>
      );
    }
        
    return (
      <Cell {...props}>
        <div style={{ display: "flex" }}>
          <div style={{ display: "flex" }} className="btn-group">
            <Link className="btn btn-xs btn-primary" to={defaultTermUrl}>
              {defaultLogin}
            </Link>                          
            <button data-toggle="dropdown"
              onClick={this.onShowLoginsClick}  
              className="btn btn-default btn-xs dropdown-toggle" aria-expanded="true">
              <span className="caret"></span>
            </button>
            <ul className="dropdown-menu pull-right">
              <li>
                <div className="input-group-sm grv-nodes-custom-login">
                  <input className="form-control" ref="customLogin"
                    placeholder="Enter login name..."
                    onKeyPress={this.onKeyPress}
                    autoFocus
                    />
                </div>  
              </li>
              {$lis}
            </ul>                                        
          </div>
        </div>
      </Cell>
    )
  }
}  

class NodeList extends React.Component {

  searchableProps = ['addr', 'hostname', 'tags'];

  constructor(props) {
    super(props);
    this.state = {        
      filter: '',
      colSortDirs: { hostname: SortTypes.DESC }
    };
  }
  
  onSortChange = (columnKey, sortDir) => {
    this.state.colSortDirs = { [columnKey]: sortDir };
    this.setState(this.state);
  }

  onFilterChange = value => {
    this.state.filter = value;
    this.setState(this.state);
  }
  
  onKeyPress = e => {
    if ( (e.key === 'Enter' || e.type === 'click') && this.refs.ssh.value) {            
      const [login, serverId] = this.refs.ssh.value.split('@');
        if (login && serverId) {
          const url = cfg.getTerminalLoginUrl({
          siteId: this.props.siteId,
          serverId,
          login
        })     
        
        history.push(url);        
      }      
    }            
  }
  
  searchAndFilterCb(targetValue, searchValue, propName){
    if(propName === 'tags'){
      return targetValue.some((item) => {
        const { role, value } = item;
        return role.toLocaleUpperCase().indexOf(searchValue) !==-1 ||
          value.toLocaleUpperCase().indexOf(searchValue) !==-1;
      });
    }
  }
  
  sortAndFilter(data) {
    const { colSortDirs } = this.state;    
    const filtered = data      
      .filter(obj => isMatch(obj, this.state.filter, {
        searchableProps: this.searchableProps,
        cb: this.searchAndFilterCb
      }));
        
    const columnKey = Object.getOwnPropertyNames(colSortDirs)[0];
    const sortDir = colSortDirs[columnKey];
    let sorted = sortBy(filtered, columnKey);
    if(sortDir === SortTypes.ASC){
      sorted = sorted.reverse();
    }

    return sorted;
  }

  render() {      
    const { nodeRecords, logins, onLoginClick } = this.props;       
    const data = this.sortAndFilter(nodeRecords);                                     
    return (
      <div className="grv-nodes m-t">                
        <div className="grv-flex grv-header" style={{ justifyContent: "space-between" }}>                    
          <h2 className="text-center no-margins"> Nodes </h2>          
          <div className="grv-flex">
            <ClusterSelector/>  
            <InputSearch onChange={this.onFilterChange} />                        
            <div className="m-l grv-search input-group input-group-sm" title="login to SSH server">
              <input ref="ssh"
                className="form-control" 
                placeholder="login@host"
                onKeyPress={this.onKeyPress}                  
              />                            
              <span className="input-group-btn"> 
                <button className="btn btn-sm btn-white" onClick={this.onKeyPress}>                  
                  <i className="fa fa-terminal text-muted"/>
                </button>            
              </span>              
              </div>
              
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
                    title="Hostname"
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
                    title="Address"
                  />
                }
                cell={<TextCell data={data}/> }
              />
              <Column
                columnKey="tags"
                header={<Cell>Labels</Cell> }
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
}

export default NodeList;
