'use strict';

var WebTunsPage = React.createClass({
  getInitialState: function(){
      return {tuns: []};
  },
  componentDidMount: function() {
      this.reload();
  },
  reload: function(){
      $.ajax({
          url: this.props.url,
          dataType: "json",
      success: function(data) {
          this.setState({tuns: data});
      }.bind(this),
      error: function(xhr, status, err) {
          console.error(this.props.url, status, err.toString());
      }.bind(this)
    });
  },
  openTunForm: function(){
      this.refs.tun.open();
  },
  addTun: function(tun){
      $.ajax({
          url: this.props.url,
          dataType: "json",
          type: "POST",
          data: tun,
          success: function(data) {
              this.refs.tun.reset();
              this.reload();
          }.bind(this),
          error: function(xhr, status, err) {
              this.refs.tun.reset();
              console.error(this.props.url, status, err.toString());
              alert(err.toString());
          }.bind(this)
      });
  },
  deleteTun: function(prefix) {
      if (!confirm("Are you sure you want to delete tun " + prefix + " ?")) {
          return;
      }
      $.ajax({
          url: this.props.url+"/"+prefix,
          type: "DELETE",
          dataType: "json",
          success: function(data) {
              this.reload();
          }.bind(this),
          error: function(xhr, status, err) {
              console.error(this.props.url, status, err.toString());
          }.bind(this)
      });
  },
  render: function() {
    return (
<div id="wrapper">
   <LeftNavBar current="webtuns"/>
   <div id="page-wrapper" className="gray-bg">
       <TopNavBar/>
       <PageHeader title="Web SSH Tunnels" url="/webtuns"/>
       <div className="wrapper wrapper-content animated fadeInRight">
            <Box>
                <WebTunsBox tuns={this.state.tuns} onOpenTunForm={this.openTunForm} onTunDelete={this.deleteTun}/>
            </Box>            
       </div>
       <PageFooter/>
   </div>
<WebTunForm ref="tun" onAddTun={this.addTun}/>
</div>
    );
  }
});

var WebTunsBox = React.createClass({
    render: function() {
        if (this.props.tuns.length == 0) {
            return (
<div className="text-center m-t-lg">
   <h1>
      Web SSH Tunnels
   </h1>     
   <p>You have no Web SSH Tunnels.</p> 
   <p>Tunnels provide password protected access to a web server on remote machine through SSH tunnel.</p>

   <br/><br/>
   <BootstrapButton className="btn-primary" onClick={this.props.onOpenTunForm}>
        <i className="fa fa-check"></i>&nbsp;Add Web Tunnel
    </BootstrapButton>
</div>);
        }
        return (
<div>
   <WebTunsTable keys={this.props.tuns} onTunDelete={this.props.onTunDelete}/>
   <BootstrapButton className="btn-primary" onClick={this.props.onOpenTunForm}>
      <i className="fa fa-check"></i>&nbsp;Add Web Tunnel
   </BootstrapButton>
</div>
);
    }
})

var WebTunForm = React.createClass({
  open: function() {
    this.refs.modal.open();
  },
  close: function() {
    this.refs.modal.close();
  },
  reset: function() {
      React.findDOMNode(this.refs.prefix).value = "";
      React.findDOMNode(this.refs.target).value = "";
      React.findDOMNode(this.refs.proxy).value = "";
      this.refs.modal.close();
  },
  confirm: function() {
      var prefix = React.findDOMNode(this.refs.prefix).value.trim();
      var target = React.findDOMNode(this.refs.target).value.trim();
      var proxy = React.findDOMNode(this.refs.proxy).value.trim();
      if (!prefix || !target || !proxy) {
          alert("Prefix, target and proxy can not be empty");
          return;
      }
      this.props.onAddTun({prefix: prefix, target: target, proxy: proxy});
  },
  render: function() {
      return (
<BootstrapModal
   icon="fa-arrows-h"
   ref="modal"
   confirm="OK"
   cancel="Cancel"
   onCancel={this.reset}
   onConfirm={this.confirm}
   title="Add Web Tunel">      
      <div className="form-group">
         <label>Subdomain</label> 
         <div className="input-group m-b">
             <input placeholder="subdomain" className="form-control" ref="prefix" type="text"/>
             <span className="input-group-addon">.gravitational.io</span>
         </div>
         <p>This subdomain will point to the remote web server when accessed. It will be protected by the standard portal password.</p>
      </div>
      
      <div className="form-group">
         <label>SSH Proxy</label> 
         <input placeholder="node.gravitational.io:2022" className="form-control" ref="proxy"/>
         <p>SSH proxy in form of host:port will be used to access the target address</p>
      </div>

      <div className="form-group">
         <label>SSH Proxy</label> 
         <input placeholder="http://localhost:8080" className="form-control" ref="target"/>
         <p>Target web server address</p>
      </div>
</BootstrapModal>
    );
  }
});

var WebTunsTable = React.createClass({
  render: function() {
    var tunDelete = this.props.onTunDelete
    var rows = this.props.keys.map(function (tun, index) {
      return (
        <WebTunRow tun={tun} key={index} onTunDelete={tunDelete}/>
      );
    });
    return (
<table className="table table-striped">
   <thead>
      <tr>
         <th>Subdomain</th>
         <th>Proxy</th>
         <th>Target</th>
         <th>Portal</th>
         <th></th>
      </tr>
   </thead>
   <tbody>
      {rows}
   </tbody>
</table>);
  }
});

var WebTunRow = React.createClass({
  handleDelete: function(e) {
      e.preventDefault();
      this.props.onTunDelete(this.props.tun.prefix);
  },
  render: function() {
    return (
<tr className="tun">
   <td>{this.props.tun.prefix}</td>
   <td>{this.props.tun.proxy}</td>
   <td>{this.props.tun.target}</td>
   <td><a href={"http://"+this.props.tun.prefix+".gravitational.io:2025"} target="_blank">{"http://"+this.props.tun.prefix+".gravitational.io:2025"}</a></td>
   <td><a href="#" onClick={this.handleDelete}><i className="fa fa-times text-navy"></i></a></td>
</tr>
    );
  }
});


React.render(
  <WebTunsPage url="/api/tunnels/web"/>,
  document.body
);
