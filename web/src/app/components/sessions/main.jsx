var React = require('react');
var reactor = require('app/reactor');
var {Table, Column, Cell, TextCell} = require('app/components/table.jsx');
var {getters} = require('app/modules/sessions');
var {open} = require('app/modules/activeTerminal/actions');

const UsersCell = ({ rowIndex, data, ...props }) => {
  var $users = data[rowIndex].parties.map((item, itemIndex)=>
    (<span key={itemIndex} className="text-uppercase label label-primary">{item.user[0]}</span>)
  )

  return (
    <Cell {...props}>
      <div>
        {$users}
      </div>
    </Cell>
  )
};

const ButtonCell = ({ rowIndex, data, ...props }) => {
  let onClick = () => {
    var rowData = data[rowIndex];
    var {sid, addr} = rowData
    var user = rowData.parties[0].user;
    open(addr, user, sid);
  }

  return (
    <Cell {...props}>
      <button onClick={onClick} className="btn btn-info btn-circle" type="button">
        <i className="fa fa-terminal"></i>
      </button>
    </Cell>
  )
}

var SessionList = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      sessionsView: getters.sessionsView
    }
  },

  render: function() {
    var data = this.state.sessionsView;
    return (
      <div className="grv-sessions">
        <h1> Sessions</h1>
        <div className="">
          <div className="">
            <div className="">
              <Table rowCount={data.length} className="table-stripped">
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
                  columnKey="addr"
                  header={<Cell> Node </Cell> }
                  cell={<TextCell data={data} /> }
                />

                <Column
                  columnKey="addr"
                  header={<Cell> Users </Cell> }
                  cell={<UsersCell data={data} /> }
                />
              </Table>
            </div>
          </div>
        </div>
      </div>
    )
  }
});

module.exports = SessionList;
