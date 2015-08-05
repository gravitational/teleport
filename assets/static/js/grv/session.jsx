'use strict';

var SessionPage = React.createClass({
    onShowEvent: function(e) {
        this.refs.event.show(e);
    },
    onServerSelect: function(addr) {
        this.current_server = addr;
        this.refs.upload.onServerSelect(this.current_server);
    },
    getCurrentServer() {
        return this.current_server || null;
    },
    getInitialState: function(){
        var state = parseSessionWithServers({
            id: session.id,
            parties: [],
            first_server: session.first_server}, []);
        state.session.events = [];
        return state;
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
              var state = parseSessionWithServers(data.session, data.servers);
              state.session.events = data.events;              
              this.setState(state);
          }.bind(this),
          error: function(xhr, status, err) {
              console.error(this.props.url, status, err.toString());
          }.bind(this)
      });
    },
    onUpload: function(){
        this.refs.upload.open();
    },
    onDownload: function(){
        this.refs.download.open();
    },    
    render: function() {
        return (
            <div id="wrapper">
              <LeftNavBar current="sessions"/>
              <div id="page-wrapper" className="gray-bg">
                <TopNavBar/>
                <div className="wrapper wrapper-content">
                  <div className="row">
                    <div className="col-lg-9" style={{width: '920px'}}>
                      <ConsoleBox session={this.state.session}
                                  onServerSelect={this.onServerSelect}
                                  onUpload={this.onUpload}
                                  onDownload={this.onDownload}/>
                    </div>
                    <div className="col-lg-6" style={{width: '920px'}}>
                      <ChatBox session={this.state.session} onShowEvent={this.onShowEvent}/>
                    </div>
                  </div>
                </div>
                <PageFooter/>
              </div>
              <UploadForm ref="upload" getCurrentServer={this.getCurrentServer}/>
              <DownloadForm ref="download" getCurrentServer={this.getCurrentServer}/>
              <EventForm ref="event"/>
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
                  <div className="media-body">
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
                <h5><i className="fa fa-user"></i>&nbsp;&nbsp;Connected users</h5>
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

var ChatBox = React.createClass({
    componentWillUpdate: function() {        
        var node = React.findDOMNode(this.refs.discussion);
        this.shouldScrollBottom = node.scrollTop + node.offsetHeight === node.scrollHeight;
    },
    componentDidUpdate: function() {
        if (this.shouldScrollBottom) {
            var node = React.findDOMNode(this.refs.discussion);
            node.scrollTop = node.scrollHeight
        }
    },
    sendMessage: function(message) {
        var box = $(React.findDOMNode(this.refs.message));
        $.ajax({
            url: grv.path("api", "sessions", session.id, "messages"),
            type: "POST",
            dataType: 'json',
            data: JSON.stringify(message),
            success: function(data) {
                box.val("");
            }.bind(this),
            error: function(xhr, status, err) {
                toastr.error("failed to send message, try again");
            }.bind(this)
        });
    },
    describeEvent: function(event, index) {
        var self = this;
        var showEvent = function(){
            self.props.onShowEvent(event);
        }
        switch (event.schema) {
            case "teleport.session":
                return {
                    icon: "fa fa-tty",
                    user: event.properties.user,
                    action: (<span> opened shell session from {event.properties.remoteaddr} on {event.properties.localaddr}&nbsp;&nbsp;
              <a onClick={showEvent}><i className="fa fa-play-circle"></i></a></span>),
                };
            case "teleport.exec":
                return {
                    icon: "fa fa-tty",
                    user: event.properties.user,
                    action: ("executed remote command '" + event.properties.command +
                             "from "+ event.properties.localaddr+"' on server '"+
                             event.properties.remoteaddr+"'")
                };
            case "teleport.message":
                return {
                    icon: "fa fa-comment",
                    user: event.properties.user,
                    action: event.properties.message
                };
        }
        return {icon: "fa fa-question text-navy", user: event.properties.user};
    },
    onMessage: function(e) {
        var key = e.nativeEvent.keyCode;
        if (key != 13) {
            return
        }
        var box = $(React.findDOMNode(this.refs.message));
        var message = box.val();
        this.sendMessage(message)
    },
    render: function() {
        var self = this;        
        var se = this.props.session;

        var users = se.parties.map(function(p, index) {
            var secondsAgo = Math.floor((new Date() - p.last_active) / 1000);
            var labelClass = secondsAgo < 60?"pull-right label label-primary": "pull-right label";
            return (
                <div className="chat-user">
                  <span className={labelClass}>{timeSince(p.last_active)} ago</span>
                  <div className="chat-user-name">
                    <i className="fa fa-user"></i>&nbsp;&nbsp;
                    <a href="#">{p.user}  &rarr; {p.server.addr}</a>
                  </div>
                </div>
            );
        });
        se.events.reverse();
        var events = se.events.map(function(e, index) {
            var d = self.describeEvent(e, index);
            var tm = new Date(e.time);
            return (
                <div className="chat-message left">
                  <div className="vertical-timeline-icon navy-bg" style={{top: 'auto', left: 'auto', position: 'relative', float: 'left'}}>
                    <i className={d.icon}/>
                  </div>
                  <div className="message">
                    <a className="message-author" href="#"> {d.user} </a>
                    <span className="message-date">{timeSince(tm)} ago</span>
                    <span className="message-content">
                      {d.action}
                    </span>
                  </div>
                </div>);
        });        

        return (
            <div className="ibox chat-view">
                <div className="ibox-title">
                  <i className="fa fa-wechat" style={{marginRight: '6px'}}/>Events and Chat
                </div>
                <div className="ibox-content">
                  <div className="row">
                    <div className="col-md-9">
                      <div className="chat-discussion" ref="discussion">
                        {events}
                      </div>
                    </div>
                    <div className="col-md-3">
                      <div className="chat-users">
                        <div className="users-list">
                          {users}
                        </div>
                      </div>
                    </div>
                  </div>
                  <div className="row">
                    <div className="col-lg-12">
                      <div className="chat-message-form">
                        <div className="form-group">
                          <textarea className="form-control message-input"
                                    ref="message"
                                    placeholder="Enter text to send message"
                                    onKeyPress={this.onMessage}></textarea>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>);

        return (
            <div className="ibox float-e-margins">
              <div className="ibox-title">
                <h5><i className="fa fa-chat-o"></i>Chat</h5>
              </div>
              <div className="ibox-content">
                <div className="feed-activity-list">
                  {events}
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
        var socket = new WebSocket("ws://"+hostport+grv.path("api", "ssh", "connect", srv, "sessions", self.props.session.id), "proto");
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
            this.props.onServerSelect(addr);
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
    <div className="pull-right">
      <button id="session-follow" className="btn btn-default" type="button" onClick={self.onFollow} style={{marginRight: '5px'}}>following activity</button>
      <div className="btn-group pull-right">
        <button data-toggle="dropdown" className="btn btn-primary dropdown-toggle">Actions <span className="caret"></span></button>
        <ul className="dropdown-menu">
          <li><a href="#" onClick={this.props.onUpload}><i className="fa fa-upload"></i>&nbsp;<span>Upload files to server</span></a></li>
          <li><a href="#" onClick={this.props.onDownload}><i className="fa fa-download"></i>&nbsp;<span>Download files from server</span></a></li>
        </ul>
      </div>
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


var UploadForm = React.createClass({
    files: function(){
        return $(React.findDOMNode(this.refs.files));
    },
    upload: function(){
        return $(React.findDOMNode(this.refs.fileupload));
    },
    onServerSelect: function(addr) {
    },
    shouldComponentUpdate: function() {
        return false;
    },
    componentDidMount: function() {
    },
    filesList: function() {
        var list = [];
        for(var i = 0; i < this.items.length; i++) {
            console.log(this.items[i]);
            for(var j = 0; j < this.items[i].files.length; j++) {
                list.push(this.items[i].files[j].name);
            }
        }
        return list;
    },
    open: function(srv) {
        this.items = [];
        this.uploaded = 0;
        var self = this;
        var upload = this.upload();
        var files = this.files();
        upload.fileupload({
            autoUpload: false,
            url: grv.path('servers', self.props.getCurrentServer(), 'files'),
            add: function (e, data) {
                self.items.push(data);
                data.context = {};
                $.each(data.files, function (index, file) {
                    var item = $('<li class="list-group-item"/>');
                    var ok = $('<i class="fa fa-check pull-right" style="display:none;"/>')
                        var handle = $('<div/>').text(file.name + " ("+formatBytes(file.size)+")");
                    handle.append(ok);
                    handle.appendTo(item);
                    item.appendTo(files);                
                    data.context[file.name] = ok;
                });
            },
            submit: function (e, data) {
                var path = $(React.findDOMNode(self.refs.path));
                data.formData = {
                    path: path.val(),
                    addr: self.props.getCurrentServer()
                };            
                if (!data.formData.path) {
                    path.focus();
                    return false;
                }
            },
            done: function (e, data) {
                self.uploaded += 1;
                for(var i = 0; i < data.files.length; ++i) {
                    $(data.context[data.files[i].name]).show();
                }
                if(self.uploaded == self.items.length) {
                    var message = "files "+ self.filesList().join() +" uploaded on "+ self.props.getCurrentServer();
                    self.onClose();
                    toastr.success(message);
                }
            },
            fail: function (e, data) {
                toastr.error("files upload failed, try again");
                self.onClose();
            },
        });
        this.refs.modal.open();
    },
    onClose: function() {
        $(React.findDOMNode(this.refs.modal)).find(":input,:button,a").attr("disabled", false);        
        this.refs.modal.setConfirmText("Upload");
        for(var i = 0; i < this.items.length; i++) {            
            //this.items[i].abort();
        }
        this.items = [];
        this.upload().fileupload('destroy');
        this.files().empty();
        this.refs.modal.close();
    },
    onUpload: function() {
        this.refs.modal.setConfirmText("Uploading...");
        $(React.findDOMNode(this.refs.modal)).find(":input,:button,a").attr("disabled", true);
        for(var i =0; i < this.items.length; i++ ) {
            this.items[i].submit();
        }
    },
    render: function() {
        return (
            <BootstrapModal dialogClass="modal-dialog" icon="fa fa-upload" ref="modal"
                            cancel="Cancel" confirm="Upload" onConfirm={this.onUpload}
                            onCancel={this.onClose} title="Upload files">
              <form method="get" className="form-horizontal">
              <div className="form-group">
                <label className="col-sm-4 control-label">Directory on server</label>
                <div className="col-sm-8">
                  <input type="text" className="form-control" defaultValue="/tmp" ref="path"></input>
                </div>
              </div>
              <div className="form-group">
                <label className="col-sm-4 control-label">Files</label>
                <div className="col-sm-8">
                  <ol className="list-group" ref="files"></ol>
                <span className="btn btn-success fileinput-button">
                  <i className="glyphicon glyphicon-plus"></i>
                  <span>Add files...</span>
                  <input ref="fileupload" type="file" name="file" multiple/>
                </span>
                </div>                
              </div>
              </form>
            </BootstrapModal>
        );
  }
});



var DownloadForm = React.createClass({
    shouldComponentUpdate: function() {
        return false;
    },
    componentDidMount: function() {
    },
    open: function(srv) {
        var self = this;
        $(React.findDOMNode(this.refs.tree)).jstree({
            'core' : {
                'check_callback' : true,
                'data' : {
                    'url' : function (node) {
                        if(node.id === "#") {
                            return grv.path('servers', self.props.getCurrentServer(), 'ls') + '?node=/';
                        }
                        return grv.path('servers', self.props.getCurrentServer(), 'ls') +'?node='+node.id;
                    },
                    'data' : function (node) {
                        return { 'id' : node.id };
                    }
                },
                'plugins' : [ 'types', 'dnd' ],
                'types' : {
                    'default' : {
                        'icon' : 'fa fa-folder'
                    },
                    'html' : {
                        'icon' : 'fa fa-file-code-o'
                    },
                    'svg' : {
                        'icon' : 'fa fa-file-picture-o'
                    },
                    'css' : {
                        'icon' : 'fa fa-file-code-o'
                    },
                    'img' : {
                        'icon' : 'fa fa-file-image-o'
                    },
                    'js' : {
                        'icon' : 'fa fa-file-text-o'
                    }
                }
            }
        });
        this.refs.modal.open();
    },
    onClose: function() {
        var tree = $(React.findDOMNode(this.refs.tree)).jstree(true);
        tree.destroy();
        this.refs.modal.close();
    },
    onDownload: function() {
        var tree = $(React.findDOMNode(this.refs.tree)).jstree(true);
        var files = tree.get_selected();
        if(files.length==0) {
            return;
        }
        var downloads = [];
        for(var i = 0; i < files.length; i++) {
            downloads.push({name: 'path', value: files[i]});
        }
        tree.destroy();
        this.refs.modal.close();
        $.fileDownload(grv.path('servers', this.props.getCurrentServer(),'download') + "?"+$.param(downloads), {
            successCallback: function (url) {
                toastr.success("files " + files.join() + "downloaded");
            },
            failCallback: function (html, url) {
                toastr.error("files " + files.join() + "failed to download");
            }
        });
    },
    render: function() {
        return (
            <BootstrapModal dialogClass="modal-dialog" icon="fa fa-download" ref="modal"
                            cancel="Cancel" confirm="Download" onConfirm={this.onDownload}
                            onCancel={this.onClose} title="Download files">
              <div ref="tree">
              </div>
            </BootstrapModal>
        );
  }
});


React.render(
  <SessionPage url={grv.path("api", "sessions", session.id)} pollInterval={2000}/>,
  document.body

);
