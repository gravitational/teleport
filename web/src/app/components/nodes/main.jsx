var React = require('react');
var reactor = require('app/reactor');
var {getters, actions} = require('app/modules/nodes');
var {Table, Column, Cell} = require('app/components/table.jsx');

const TextCell = ({rowIndex, data, columnKey, ...props}) => (
  <Cell {...props}>
    {data[rowIndex][columnKey]}
  </Cell>
);

var Nodes = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      nodeRecords: getters.nodeListView
    }
  },

  componentDidMount(){
    actions.fetchNodes();
  },

  renderRows(){
  },

  render: function() {
    var data = this.state.nodeRecords;
    return (
      <div>
        <h1> Nodes </h1>
        <div className="">
          <div className="">
            <div className="">
              <Table rowCount={data.length}>
                <Column
                  columnKey="count"
                  header={<Cell> Sessions </Cell> }
                  cell={<TextCell data={data}/> }
                />
                <Column
                  columnKey="ip"
                  header={<Cell> Node </Cell> }
                  cell={<TextCell data={data}/> }
                />
                <Column
                  columnKey="tags"
                  header={<Cell></Cell> }
                  cell={<TextCell data={data}/> }
                />
                <Column
                  columnKey="roles"
                  header={<Cell>Login as</Cell> }
                  cell={<TextCell data={data}/> }
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
