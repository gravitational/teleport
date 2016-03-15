var React = require('react');
var reactor = require('app/reactor');
var PureRenderMixin = require('react-addons-pure-render-mixin');
var {lastMessage} = require('app/modules/notifications/getters');
var {ToastContainer, ToastMessage} = require("react-toastr");
var ToastMessageFactory = React.createFactory(ToastMessage.animation);

var NotificationHost = React.createClass({

  mixins: [
    reactor.ReactMixin, PureRenderMixin
  ],

  getDataBindings() {
    return {msg: lastMessage}
  },

  update(msg) {
    if (msg) {
      if (msg.isError) {
        this.refs.container.error(msg.text, msg.title);
      } else if (msg.isWarning) {
        this.refs.container.warning(msg.text, msg.title);
      } else if (msg.isSuccess) {
        this.refs.container.success(msg.text, msg.title);
      } else {
        this.refs.container.info(msg.text, msg.title);
      }
    }
  },

  componentDidMount() {
    reactor.observe(lastMessage, this.update)
  },

  componentWillUnmount() {
    reactor.unobserve(lastMessage, this.update);
  },

  render: function() {
    return (
        <ToastContainer ref="container" toastMessageFactory={ToastMessageFactory} className="toast-top-right"/>
    );
  }
});

module.exports = NotificationHost;
