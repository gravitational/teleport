var React = require('react');
var reactor = require('app/reactor');
var {getters, actions} = require('app/modules/nodes');
var userGetters = require('app/modules/user/getters');
var {Table, Column, Cell} = require('app/components/table.jsx');
var {createNewSession} = require('app/modules/activeTerminal/actions');

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

const LoginCell = ({user, onLoginClick, rowIndex, data, ...props}) => {
  if(!user || user.logins.length === 0){
    return <Cell {...props} />;
  }

  var serverId = data[rowIndex].id;
  var $lis = [];

  function onClick(i){
    var login = user.logins[i];
    if(onLoginClick){
      return ()=> onLoginClick(serverId, login);
    }else{
      return () => createNewSession(serverId, login);
    }
  }

  for(var i = 0; i < user.logins.length; i++){
    $lis.push(<li key={i}><a onClick={onClick(i)}>{user.logins[i]}</a></li>);
  }

  return (
    <Cell {...props}>
      <div className="btn-group">
        <button type="button" onClick={onClick(0)} className="btn btn-xs btn-primary">{user.logins[0]}</button>
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
    var onLoginClick = this.props.onLoginClick;
    return (
      <div className="grv-nodes">
        <h1> Nodes </h1>
        <div className="">
          <div className="">
            <div className="">
              <Table rowCount={data.length} className="table-striped grv-nodes-table">
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
                  onLoginClick={onLoginClick}
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
