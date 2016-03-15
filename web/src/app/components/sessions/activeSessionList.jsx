var React = require('react');
var reactor = require('app/reactor');
var {DateRangePicker, CalendarNav} = require('./../datePicker.jsx');
var {Table, Column, Cell, TextCell, SortHeaderCell, SortTypes} = require('app/components/table.jsx');
var {ButtonCell, UsersCell, EmptyList, NodeCell, DurationCell, DateCreatedCell} = require('./listItems');

var ActiveSessionList = React.createClass({
  render: function() {
    let data = this.props.data.filter(item => item.active);
    return (
      <div className="grv-sessions-active">
        <div className="grv-header">
          <h1> Active Sessions </h1>
        </div>
        <div className="grv-content">
          {data.length === 0 ? <EmptyList text="You have no active sessions."/> :
            <div className="">
              <Table rowCount={data.length} className="table-striped">
                <Column
                  columnKey="sid"
                  header={<Cell> Session ID </Cell> }
                  cell={<TextCell data={data}/> }
                />
                <Column
                  header={<Cell> </Cell> }
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
