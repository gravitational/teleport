var cfg = require('app/config');
var React = require('react');

var EventStreamer = React.createClass({
  componentDidMount() {
    var {token, sid} = this.props;
    var connStr = cfg.api.getEventStreamerConnStr(token, sid);

    this.socket = new WebSocket(connStr, "proto");
    this.socket.onmessage = () => {};
    this.socket.onclose = () => {};
  },

  componentWillUnmount() {
    this.socket.close();
  },

  shouldComponentUpdate(){
    return false;
  },

  render() {
    return null;
  }
});

export default EventStreamer;
