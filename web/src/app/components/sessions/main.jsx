var React = require('react');
var reactor = require('app/reactor');
var Nodes = React.createClass({
  render: function() {
    return (
      <div>
        <h1> Sessions </h1>
        <div className="">
          <div className="">
            <div className="">
              <table className="table table-striped">
                <thead>
                  <tr>
                    <th>Node</th>
                    <th>Status</th>
                    <th>Labels</th>
                      <th>CPU</th>
                      <th>RAM</th>
                      <th>OS</th>
                      <th> Last Heartbeat </th>
                    </tr>
                  </thead>
                <tbody></tbody>
              </table>
            </div>
          </div>
        </div>
      </div>
    )
  }
});

module.exports = Nodes;
