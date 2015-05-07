'use strict';

var ServersPage = React.createClass({
  getInitialState: function(){
      return {servers: []};
  },
  componentDidMount: function() {
      this.reload();
      setInterval(this.reload, this.props.pollInterval);
  },
  connect: function(srv){
      this.refs.term.connect(srv);
  },
  disconnect: function(srv){
  },
  reload: function(){
    $.ajax({
      url: this.props.url,
      dataType: 'json',
      success: function(data) {
        if(this.modalOpen == true) {
           return
        }
        this.setState({servers: data});
      }.bind(this),
      error: function(xhr, status, err) {
        console.error(this.props.url, status, err.toString());
      }.bind(this)
    });
  },
  render: function() {
    return (
<div id="wrapper">
   <LeftNavBar current="servers"/>
   <div id="page-wrapper" className="gray-bg">
       <TopNavBar/>
       <PageHeader title="SSH Servers" url="/servers"/>
       <div className="wrapper wrapper-content animated fadeInRight">
            <Box>
                <ServersTable servers={this.state.servers}  onConnect={this.connect}/>
            </Box>            
       </div>
       <PageFooter/>
   </div>
   <ServerForm ref="term" onDisconnect={this.disconnect}/>
</div>
    );
  }
});


var ServersTable = React.createClass({
  render: function() {
    var onConnect = this.props.onConnect
    var onDisconnect = this.props.onDisconnect
    var rows = this.props.servers.map(function (srv, index) {
      return (
        <ServerRow srv={srv} key={index} onConnect={onConnect} onDisconnect={onDisconnect}/>
      );
    });
    return (
<table className="table table-striped">
   <thead>
      <tr>
         <th></th>
         <th>Address</th>
      </tr>
   </thead>
   <tbody>
      {rows}
   </tbody>
</table>);
  }
});


var ServerRow = React.createClass({
  handleConnect: function(e) {
      e.preventDefault();
      this.props.onConnect(this.props.srv);
  },
  render: function() {
    return (
<tr>
   <td><a href="#" onClick={this.handleConnect}><i className="fa fa-tty text-navy"></i></a></td>
   <td><a href="#" onClick={this.handleConnect}>{this.props.srv.addr}</a></td>
</tr>
    );
  }
});

var ServerForm = React.createClass({
  connect: function(srv) {
      var self = this;
      var hostport = location.hostname+(location.port ? ':'+location.port: '');
      var socket = new WebSocket("ws://"+hostport+"/api/ssh/connect/"+srv.addr, "proto");
      socket.onopen = function() {
          self.term = new Terminal({
              cols: 120,
              rows: 24,
              useStyle: true,
              screenKeys: true,
              cursorBlink: false
          });

          self.term.on('data', function(data) {
              socket.send(data);
          });

          self.term.open(React.findDOMNode(self.refs.term));
          self.term.write('\x1b[31mWelcome to teleport!\x1b[m\r\n');

          socket.onmessage = function(e) {
              self.term.write(e.data);
          }
          socket.onclose = function() {
              self.close()
          }
      }
      this.refs.modal.open();
  },
  close: function() {
      if (this.term != null) {
          this.term.destroy();
          this.term = null;
      }
      this.refs.modal.close();
      this.props.onDisconnect();
  },
  render: function() {
      return (
      <BootstrapModal
        dialogClass="modal-lg"
        icon="fa-tty"
        ref="modal"
        cancel="Close"
        onCancel={this.close}
        title="SSH Console">
          <div ref="term" style={{width: '580px', height: '400px'}} className="text-center m-t-lg"></div>
      </BootstrapModal>
    );
  }
});

React.render(
  <ServersPage url="/api/servers" pollInterval={2000}/>,
  document.body
);
