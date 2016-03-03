var cfg = require('app/config');
var React = require('react');
var session = require('app/session');
var {updateSession} = require('app/modules/sessions/actions');

var EventStreamer = React.createClass({
  componentDidMount() {
    let {sid} = this.props;
    let {token} = session.getUserData();
    let connStr = cfg.api.getEventStreamConnStr(token, sid);

    this.socket = new WebSocket(connStr, 'proto');
    this.socket.onmessage = (event) => {
      try
      {
        let json = JSON.parse(event.data);
        updateSession(json.session);
      }
      catch(err){
        console.log('failed to parse event stream data');
      }

    };
    this.socket.onclose = () => {};
  },

  componentWillUnmount() {
    this.socket.close();
  },

  shouldComponentUpdate() {
    return false;
  },

  render() {
    return null;
  }
});

export default EventStreamer;
