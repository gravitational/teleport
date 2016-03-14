var React = require('react');
var { Link } = require('react-router');
var LinkedStateMixin = require('react-addons-linked-state-mixin');
var {Table, Column, Cell, TextCell, SortHeaderCell, SortTypes} = require('app/components/table.jsx');
var {open} = require('app/modules/activeTerminal/actions');
var moment =  require('moment');
var {isMatch} = require('app/common/objectUtils');
var _ = require('_');

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

  mixins: [LinkedStateMixin],

  getInitialState(props){
    this.searchableProps = ['serverIp', 'created', 'active'];
    return { filter: '', colSortDirs: {} };
  },

  onSortChange(columnKey, sortDir) {
    this.setState({
      ...this.state,
      colSortDirs: { [columnKey]: sortDir }
    });
  },

  searchAndFilterCb(targetValue, searchValue, propName){
    if(propName === 'created'){
      var displayDate = moment(targetValue).fromNow().toLocaleUpperCase();
      return displayDate.indexOf(searchValue) !== -1;
    }
  },

  sortAndFilter(data){
    var filtered = data.filter(obj=>
      isMatch(obj, this.state.filter, {
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
    var data = this.sortAndFilter(this.props.sessionRecords);
    return (
      <div>
        <div className="grv-search">
          <input valueLink={this.linkState('filter')} placeholder="Search..." className="form-control input-sm"/>
        </div>
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
              header={
                <SortHeaderCell
                  sortDir={this.state.colSortDirs.serverIp}
                  onSortChange={this.onSortChange}
                  title="Node"
                />
              }
              cell={<TextCell data={data} /> }
            />
            <Column
              columnKey="created"
              header={
                <SortHeaderCell
                  sortDir={this.state.colSortDirs.created}
                  onSortChange={this.onSortChange}
                  title="Created"
                />
              }
              cell={<DateCreatedCell data={data}/> }
            />
            <Column
              columnKey="active"
              header={
                <SortHeaderCell
                  sortDir={this.state.colSortDirs.active}
                  onSortChange={this.onSortChange}
                  title="Active"
                />
              }
              cell={<UsersCell data={data} /> }
            />
          </Table>
        </div>
      </div>
    )
  }
});

module.exports = SessionList;
