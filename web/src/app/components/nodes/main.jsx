var React = require('react');
var reactor = require('app/reactor');
var {getters, actions} = require('app/modules/nodes');
var userGetters = require('app/modules/user/getters');
var {Table, Column, Cell} = require('app/components/table.jsx');
var {open} = require('app/modules/activeTerminal/actions');

const TextCell = ({rowIndex, data, columnKey, ...props}) => (
  <Cell {...props}>
    {data[rowIndex][columnKey]}
  </Cell>
);

const TagCell = ({rowIndex, data, columnKey, ...props}) => (
  <Cell {...props}>
    { data[rowIndex].tags.map((item, index) =>
      (<span key={index} className="label label-default">
        {item.role} <li className="fa fa-long-arrow-right"></li>
        {item.value}
      </span>)
    ) }
  </Cell>
);

const LoginCell = ({user, rowIndex, data, ...props}) => {
  if(!user || user.logins.length === 0){
    return <Cell {...props} />;
  }

  var $lis = [];

  for(var i = 0; i < user.logins.length; i++){
    $lis.push(<li key={i}><a href="#" target="_blank" onClick={open.bind(null, data[rowIndex].id, user.logins[i], undefined)}>{user.logins[i]}</a></li>);
  }

  return (
    <Cell {...props}>
      <div className="btn-group">
        <button type="button" onClick={open.bind(null, data[rowIndex].id, user.logins[0], undefined)} className="btn btn-sm btn-primary">{user.logins[0]}</button>
        {
          $lis.length > 1 ? (
            <div className="btn-group">
              <button data-toggle="dropdown" className="btn btn-default btn-sm dropdown-toggle" aria-expanded="true">
                <span className="caret"></span>
              </button>
              <ul className="dropdown-menu">
                <li><a href="#" target="_blank">Logs</a></li>
                <li><a href="#" target="_blank">Logs</a></li>
              </ul>
            </div>
          ): null
        }
      </div>
    </Cell>
  )
};

var Nodes = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      nodeRecords: getters.nodeListView,
      user: userGetters.user
    }
  },

  render: function() {
    var data = this.state.nodeRecords;
    return (
      <div className="grv-nodes">
        <h1> Nodes </h1>
        <div className="">
          <div className="">
            <div className="">
              <Table rowCount={data.length} className="table-stripped grv-nodes-table">
                <Column
                  columnKey="sessionCount"
                  header={<Cell> Sessions </Cell> }
                  cell={<TextCell data={data}/> }
                />
                <Column
                  columnKey="addr"
                  header={<Cell> Node </Cell> }
                  cell={<TextCell data={data}/> }
                />
                <Column
                  columnKey="tags"
                  header={<Cell></Cell> }
                  cell={<TagCell data={data}/> }
                />
                <Column
                  columnKey="roles"
                  header={<Cell>Login as</Cell> }
                  cell={<LoginCell data={data} user={this.state.user}/> }
                />
              </Table>
            </div>
          </div>
        </div>
      </div>
    )
  }
});

module.exports = Nodes;
