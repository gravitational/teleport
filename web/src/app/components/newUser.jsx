var React = require('react');
var $ = require('jQuery');
var reactor = require('app/reactor');
var {actions, getters} = require('app/modules/invite');
var userModule = require('app/modules/user');
var LinkedStateMixin = require('react-addons-linked-state-mixin');

var InviteInputForm = React.createClass({

  mixins: [LinkedStateMixin],

  getInitialState() {
    return {
      name: this.props.invite.user,
      psw: '',
      pswConfirmed: '',
      token: ''
    }
  },

  onClick: function(e) {
    e.preventDefault();
    //if (this.isValid()) {
      userModule.actions.signUp({
        name: this.state.name,
        psw: this.state.psw,
        token: this.state.token,
        inviteToken: this.props.invite.invite_token});
    //}
  },

  isValid: function() {
    var $form = $(".loginscreen form");
    return $form.length === 0 || $form.valid();
  },

  render() {
    return (
      <div>
        <h3> Get started with teleport </h3>
        <div className="">
          <div className="form-group">
            <input className="form-control" placeholder="Username" valueLink={this.linkState('name')}/>
          </div>
          <div className="form-group">
            <input type="password" className="form-control" placeholder="Password" valueLink={this.linkState('psw')}/>
          </div>
          <div className="form-group">
            <input type="password" className="form-control" placeholder="Password confirm"  valueLink={this.linkState('pswConfirmed')}/>
          </div>
          <div className="form-group">
            <input className="form-control" placeholder="Two factor token (Google Authenticator)"  valueLink={this.linkState('token')}/>
          </div>
          <button type="submit" className="btn btn-primary block full-width m-b" onClick={this.onClick} >Sign up</button>
        </div>
      </div>
    );
  }
})

var Invite = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      invite: getters.invite,
      attemp: getters.attemp
    }
  },

  componentDidMount(){
    actions.fetchInvite(this.props.params.inviteToken);
  },

  render: function() {
    var isProcessing = false; //this.state.userRequest.get('isLoading');
    var isError = false; //this.state.userRequest.get('isError');

    if(!this.state.invite) {
      return null;
    }

    return (
      <div className="grv grv-invite text-center">
        <div className="grv-logo-tprt"></div>
        <div className="grv-content grv-flex">
          <div className="grv-flex-column">
            <InviteInputForm invite={this.state.invite.toJS()}/>
          </div>
          <div className="grv-flex-column">
            <h4>Scan bar code for auth token <br/> <small>Scan below to generate your two factor token</small></h4>
            <img className="img-thumbnail" src={ `data:image/png;base64,${this.state.invite.get('qr')}` } />
          </div>
        </div>
      </div>
    );
  }
});

module.exports = Invite;
