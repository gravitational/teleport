var React = require('react');
var NavLeftBar = require('./navLeftBar');

var App = React.createClass({
  render: function(){
    return (
      <div className="grv">
        <NavLeftBar/>
        <div style={{'marginLeft': '100px'}}>
          {this.props.children}
        </div>
      </div>
    );
  }
})

module.exports = App;
