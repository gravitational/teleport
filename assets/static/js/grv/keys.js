'use strict';

var KeysPage = React.createClass({
  getInitialState: function(){
      return {keys: []};
  },
  componentDidMount: function() {
      this.reload();
  },
  reload: function(){
      $.ajax({
          url: this.props.url,
          dataType: "json",
      success: function(data) {
          this.setState({keys: data});
      }.bind(this),
      error: function(xhr, status, err) {
          console.error(this.props.url, status, err.toString());
      }.bind(this)
    });
  },
  openKeyForm: function(){
      this.refs.key.open();
  },
  addKey: function(key){
      $.ajax({
          url: this.props.url,
          dataType: "json",
          type: "POST",
          data: key,
          success: function(data) {
              this.refs.key.reset();
              this.refs.cert.show(atob(data["value"]));
              this.reload();
          }.bind(this),
          error: function(xhr, status, err) {
              this.refs.key.reset();
              console.error(this.props.url, status, err.toString());
              alert(err.toString());
          }.bind(this)
      });
  },
  deleteKey: function(key) {
      if (!confirm("Are you sure you want to delete key " + key + " ?")) {
          return;
      }
      $.ajax({
          url: this.props.url+"/"+key,
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
   <LeftNavBar current="keys"/>
   <div id="page-wrapper" className="gray-bg">
       <TopNavBar/>
       <PageHeader title="SSH Keys" url="/keys"/>
       <div className="wrapper wrapper-content animated fadeInRight">
            <Box>
              <KeysBox keys={this.state.keys} onOpenKeyForm={this.openKeyForm} onKeyDelete={this.deleteKey}/>
            </Box>            
       </div>
       <PageFooter/>
   </div>
<SignForm ref="key" onAddKey={this.addKey}/>
<CertView ref="cert"/>
</div>
    );
  }
});



var KeysBox = React.createClass({
    render: function() {
        if (this.props.keys.length == 0) {
            return (
<div className="text-center m-t-lg">
   <h1>
      Public SSH Keys.
   </h1>     
   <small>You have no SSH Keys added. To get an access to cluster,add and sign your public key here.</small><br/><br/>
   <BootstrapButton className="btn-primary" onClick={this.props.onOpenKeyForm}>
        <i className="fa fa-check"></i>&nbsp;Add Key
    </BootstrapButton>
</div>);
        }
        return (
<div>
   <KeysTable keys={this.props.keys} onKeyDelete={this.props.onKeyDelete}/>
   <BootstrapButton className="btn-primary" onClick={this.props.onOpenKeyForm}>
      <i className="fa fa-check"></i>&nbsp;Add Key
   </BootstrapButton>
</div>
);
    }
})

var SignForm = React.createClass({
  open: function() {
    this.refs.modal.open();
  },
  close: function() {
    this.refs.modal.close();
  },
  reset: function() {
      React.findDOMNode(this.refs.id).value = "";
      React.findDOMNode(this.refs.key).value = "";
      this.refs.modal.close();
  },
  confirm: function() {
      var id = React.findDOMNode(this.refs.id).value.trim();
      var key = React.findDOMNode(this.refs.key).value.trim();
      if (!id || !key) {
          alert("ID and Key can not be empty");
          return;
      }
      this.props.onAddKey({id: id, value: key});
  },
  render: function() {
      return (
<BootstrapModal
   icon="fa-laptop"
   dialogClass="modal-lg"
   ref="modal"
   confirm="OK"
   cancel="Cancel"
   onCancel={this.reset}
   onConfirm={this.confirm}
   title="Add and Sign SSH Key">
      <div className="form-group">
         <label>Key ID</label> 
         <input placeholder="Unique ID for the Key" className="form-control" ref="id"/>
      </div>
      <div className="form-group">
         <label>Public Key</label> 
         <textarea placeholder="Paste your public key here" className="form-control" ref="key" rows="8">
         </textarea>
         <p>Once submitted, public key will be signed by this cluster certificate authority, 
            and you will get the signed certificate back. Take this certificate and add it 
            alongside to your key in <strong>user-cert.pub</strong>
         </p>
      </div>
</BootstrapModal>
    );
  }
});


var CertView = React.createClass({
  show: function(cert) {
    React.findDOMNode(this.refs.key).value = cert;
    this.refs.modal.open();
  },
  reset: function() {
      React.findDOMNode(this.refs.key).value = "";
      this.refs.modal.close();
  },
  render: function() {
      return (
<BootstrapModal
   icon="fa-laptop"
   dialogClass="modal-lg"
   ref="modal"
   cancel="Close"
   onCancel={this.reset}
   title="Signed SSH Certificate">
      <p>Congratulations! You can find the certificate below.</p>
      <p><strong>Copy this certificate</strong> (click on the textarea and press "Ctrl-A" and "Ctrl-C") 
          and save it alongside with your public key, for example to the file <strong>username-cert.pub</strong>. 
      </p>
      <p>This will allow your SSH client can use it to authenticate with the cluster.</p>
      <div className="form-group">
         <label>Signed SSH Certificate</label> 
         <textarea className="form-control" ref="key" rows="8" id="signed-ssh-cert-val"></textarea>
      </div>
</BootstrapModal>
    );
  }
});


var KeysTable = React.createClass({
  render: function() {
    var keyDelete = this.props.onKeyDelete
    var rows = this.props.keys.map(function (key, index) {
      return (
        <KeyRow id={key.id} value={key.value} key={index} onKeyDelete={keyDelete}></KeyRow>
      );
    });
    return (
<table className="table table-striped">
   <thead>
      <tr>
         <th>Key ID</th>
         <th>Key</th>
         <th>Action</th>
      </tr>
   </thead>
   <tbody>
      {rows}
   </tbody>
</table>);
  }
});

var KeyRow = React.createClass({
  handleDelete: function(e) {
      e.preventDefault();
      this.props.onKeyDelete(this.props.id);
  },
  render: function() {
    return (
<tr className="key">
   <td>{this.props.id}</td>
   <td>{atob(this.props.value).substring(0, 100)}...</td>
   <td><a href="#" onClick={this.handleDelete}><i className="fa fa-times text-navy"></i></a></td>
</tr>
    );
  }
});



React.render(
  <KeysPage url="/api/keys"/>,
  document.body
);
