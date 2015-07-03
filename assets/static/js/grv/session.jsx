'use strict';

var SessionPage = React.createClass({
    getInitialState: function(){        
        return parseSessionWithServers({id: session.id, parties: [], first_server: session.first_server}, []);
    },
    componentDidMount: function() {
        this.reload();
        setInterval(this.reload, this.props.pollInterval);
    },
    reload: function(){
      $.ajax({
          url: this.props.url,
          dataType: 'json',
          success: function(data) {
              if(this.modalOpen == true) {
                  return
              }
              this.setState(parseSessionWithServers(data.session, data.servers));
          }.bind(this),
          error: function(xhr, status, err) {
              console.error(this.props.url, status, err.toString());
          }.bind(this)
      });
    },
    render: function() {
        return (
            <div id="wrapper">
              <LeftNavBar current="sessions"/>
              <div id="page-wrapper" className="gray-bg">
                <TopNavBar/>
                <PageHeader title={"Session '"+session.id+"'"} url={"/sessions" + session.id}/>
                <div className="wrapper wrapper-content animated fadeInRight">
                  <div className="row">
                    <div className="col-lg-9" style={{width: '920px'}}>
                      <ConsoleBox session={this.state.session}/>
                    </div>
                    <div className="col-lg-3">
                      <ActivityBox session={this.state.session}/>
                    </div>
                  </div>
                </div>
                <PageFooter/>
              </div>
            </div>
        );
    }
});

var ActivityBox = React.createClass({
    render: function() {
        var self = this;        
        var se = this.props.session;
        var parties = se.parties.map(function(p, index) {
            return (
                <div className="feed-element">
                  <a href="#" className="pull-left">
                    <i className="fa fa-user"></i>
                  </a>
                  <div className="media-body=">
                    <small className="pull-right">{timeSince(p.last_active)} ago</small>
                    <strong>{p.user}</strong> typed in <strong>{p.server.addr}</strong> server.<br/>
                    <small className="text-muted">{p.last_active.toLocaleString()}</small>
                  </div>
                </div>
            );
        });

        return (
            <div className="ibox float-e-margins">
              <div className="ibox-title">
                <h5>Active Users</h5>
              </div>
              <div className="ibox-content">
                <div className="feed-activity-list">
                  {parties}
                </div>
              </div>
            </div>
        );
    }
});

var ConsoleBox = React.createClass({
    term_id: function(addr) {
        return "term-"+addr.replace(".", "_").replace(":", "_");
    },
    termNode: function(id) {
        return $("#"+this.term_id(id));
    },
    connect: function(sid, prev_srv, srv) {
        var self = this;
        if(prev_srv != null) {
            (this.termNode(prev_srv)).toggle();
        }
        if(self.terms.hasOwnProperty(srv)) {
            (this.termNode(srv)).toggle();
            return
        }
        var parent = React.findDOMNode(self.refs.terms);
        $(parent).append('<div id="'+self.term_id(srv)+'"></div>');
        var termNode = document.getElementById(this.term_id(srv));
        var hostport = location.hostname+(location.port ? ':'+location.port: '');
        var socket = new WebSocket("ws://"+hostport+"/api/ssh/connect/"+srv+"/sessions/"+self.props.session.id, "proto");
        var term = new Terminal({
            cols: 120,
            rows: 32,
            useStyle: true,
            screenKeys: true,
            cursorBlink: false
        });
        self.terms[srv] = term;
        term.open(termNode);
        term.write('\x1b[94mconnecting to "'+srv+'"\x1b[m\r\n');
        
        socket.onopen = function() {
            term.on('data', function(data) {
                socket.send(data);
            });
            socket.onmessage = function(e) {
                term.write(e.data);
            }
            socket.onclose = function() {
                term.write('\x1b[31mdisconnected\x1b[m\r\n');
            }
        }
    },
    componentDidMount: function() {
        this.current_server = null;
        this.selected_server = null;
        this.terms = {};
        this.select().chosen({}).change(this.onSelect);
        this.drawSelect(this.props.session);
        if(this.props.session.first_server != "") {
            this.connectToServer(this.props.session.first_server);
        }
    },
    shouldComponentUpdate: function() {
        return false;
    },
    componentWillReceiveProps: function(props) {
        if(props.session.servers.length == 0) {
            return
        }
        if(this.selected_server != null) {
            return
        }
        var last_active = props.session.servers[0];
        this.connectToServer(last_active.addr);
        this.drawSelect(props.session);
    },
    connectToServer: function(addr) {
        if (this.current_server != addr && addr != "") {
            var prev = this.current_server;
            this.current_server = addr;
            this.connect(this.props.session.id, prev, addr);
        }        
    },
    onSelect: function(e) {
        if(e.target.value == "") {
            return
        }
        this.selected_server = e.target.value;
        this.connectToServer(this.selected_server);
        $("#session-follow").removeClass("btn-primary");
        $("#session-follow").addClass("btn-white");        
    },
    drawSelect: function(se) {
        var s = this.select();
        s.empty();
        s.append('<option value="">Connect to server</option>');        
        for( var i = 0; i < se.servers.length; ++i) {
            var srv = se.servers[i];
            var selected = this.current_server == srv.addr ? ' selected="selected"': "";
            s.append('<option value="'+srv.addr+'" ' + selected + '>'+srv.addr+'</option>');
        }
        s.trigger("chosen:updated");
    },
    select: function() {
        return $(React.findDOMNode(this.refs.select));
    },
    toggleFollow: function() {
        $("#session-follow").toggleClass("btn-primary");
        $("#session-follow").toggleClass("btn-white");
        if(this.selected_server != null) {
            this.selected_server = null;
        } else {
            this.selected_server = this.current_server;
        }        
    },
    onFollow: function(e) {
        this.toggleFollow();
    },
    render: function() {
        var self = this;        
        var se = this.props.session;
        var servers = se.servers.map(function(s, index) {
            return (<option value={s.addr}>{s.addr}</option>);
        });

        return (
<div className="ibox float-e-margins">
  <div className="ibox-title">
    <select data-placeholder="Connect to server..." className="chosen-select" ref="select" style={{width: '350px'}}>
    </select>
    <div className="btn-group pull-right">
      <button id="session-follow" className="btn btn-primary" type="button" onClick={self.onFollow}>watching activity</button>
    </div>
  </div>
  <div className="ibox-content">
    <div className="row m-t-sm">
       <div className="col-lg-12" ref="terms"></div>
    </div>
  </div>
</div>
        );
    }
});

React.render(
  <SessionPage url={"/api/sessions/"+session.id} pollInterval={2000}/>,
  document.body
);
