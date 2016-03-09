var React = require('react');
var reactor = require('app/reactor');
var { Link } = require('react-router');
var {Table, Column, Cell, TextCell} = require('app/components/table.jsx');
var {getters} = require('app/modules/sessions');
var {open} = require('app/modules/activeTerminal/actions');
var moment =  require('moment');
var PureRenderMixin = require('react-addons-pure-render-mixin');

const DateCreatedCell = ({ rowIndex, data, ...props }) => {
  var created = data[rowIndex].created;
  var displayDate = moment(created).fromNow();
  return (
    <Cell {...props}>
      { displayDate }
    </Cell>
  )
};

const DurationCell = ({ rowIndex, data, ...props }) => {
  var created = data[rowIndex].created;
  var lastActive = data[rowIndex].lastActive;

  var end = moment(created);
  var now = moment(lastActive);
  var duration = moment.duration(now.diff(end));
  var displayDate = duration.humanize();

  return (
    <Cell {...props}>
      { displayDate }
    </Cell>
  )
};

const UsersCell = ({ rowIndex, data, ...props }) => {
  var $users = data[rowIndex].parties.map((item, itemIndex)=>
    (<span key={itemIndex} style={{backgroundColor: '#1ab394'}} className="text-uppercase grv-rounded label label-primary">{item.user[0]}</span>)
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
  var { sessionUrl, active } = data[rowIndex];
  var [actionText, actionClass] = active ? ['join', 'btn-warning'] : ['play', 'btn-primary'];
  return (
    <Cell {...props}>
      <Link to={sessionUrl} className={"btn " +actionClass+ " btn-xs"} type="button">{actionText}</Link>
    </Cell>
  )
}

var SessionList = React.createClass({

  mixins: [reactor.ReactMixin, PureRenderMixin],

  componentWillReceiveProps(nextProps) {
    //this.setState({ });
  },

  getInitialState(){
    debugger;
  /*  this._dataList = new FakeObjectDataListStore(2000);
    this._defaultSortIndexes = [];

    var size = this._dataList.getSize();
    for (var index = 0; index < size; index++) {
      this._defaultSortIndexes.push(index);
    }

    this.state = {
      sortedDataList: this._dataList,
      colSortDirs: {}
    };

    this._onSortChange = this._onSortChange.bind(this);
*/

  },

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
                  columnKey="serverIp"
                  header={<Cell> Node </Cell> }
                  cell={<TextCell data={data} /> }
                />
                <Column
                  columnKey="created"
                  header={<Cell> Created </Cell> }
                  cell={<DateCreatedCell data={data}/> }
                />
                <Column
                  columnKey="serverId"
                  header={<Cell> Active </Cell> }
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
