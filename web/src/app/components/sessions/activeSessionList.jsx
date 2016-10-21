/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
You may obtain a copy of the License at
you may not use this file except in compliance with the License.

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

var React = require('react');
var {Table, Column, Cell, TextCell, EmptyIndicator} = require('app/components/table.jsx');
var {ButtonCell, UsersCell, NodeCell, DateCreatedCell} = require('./listItems');

var ActiveSessionList = React.createClass({

  render() {
    let data = this.props.data.filter(item => item.active);

    return (
      <div className="grv-sessions-active">
        <div className="grv-header">
          <h2 className="text-center"> Active Sessions </h2>
        </div>
        <div className="grv-content">
          {data.length === 0 ? <EmptyIndicator text="You have no active sessions."/> :
            <div className="">
              <Table rowCount={data.length} className="table-striped">
                <Column
                  columnKey="sid"
                  header={<Cell> Session ID </Cell> }
                  cell={<TextCell data={data}/> }
                />
                <Column
                  header={<Cell /> }
                  cell={
                    <ButtonCell data={data} />
                  }
                />
                <Column
                  header={<Cell> Node </Cell> }
                  cell={<NodeCell data={data} /> }
                />
                <Column
                  columnKey="created"
                  header={<Cell> Created </Cell> }
                  cell={<DateCreatedCell data={data}/> }
                />
                <Column
                  header={<Cell> Users </Cell> }
                  cell={<UsersCell data={data} /> }
                />
              </Table>
            </div>
          }
        </div>
      </div>
    )
  }
});

module.exports = ActiveSessionList;
