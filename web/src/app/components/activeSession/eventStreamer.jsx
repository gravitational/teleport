var cfg = require('app/config');
var React = require('react');
var session = require('app/session');

var EventStreamer = React.createClass({
  componentDidMount() {
    let {sid} = this.props;
    let {token} = session.getUserData();
    let connStr = cfg.api.getEventStreamerConnStr(token, sid);

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
