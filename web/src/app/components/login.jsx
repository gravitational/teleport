var React = require('react');
var $ = require('jQuery');
var reactor = require('app/reactor');

var Login = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
  //    userRequest: getters.userRequest
    }
  },

  handleSubmit: function(e){
    e.preventDefault();
    if(this.isValid()){
      var loc = this.props.location;
      var email = this.refs.email.value;
      var pass = this.refs.pass.value;
      var redirect = '/web';

      if(loc.state && loc.state.redirectTo){
        redirect = loc.state.redirectTo;
      }

      //actions.login(email, pass, redirect);
    }
  },

  isValid: function(){
     var $form = $(".loginscreen form");
     return $form.length === 0 || $form.valid();
  },

  render: function() {
    var isProcessing = false;//this.state.userRequest.get('isLoading');
    var isError = false;//this.state.userRequest.get('isError');

    return (
      <div className="middle-box text-center loginscreen">
        <form>
          <div>
            <h1 className="logo-name">G</h1>
          </div>
          <h3> Welcome to Gravitational</h3>
          <p> Login in.</p>
          <div className="m-t" role="form" onSubmit={this.handleSubmit}>
            <div className="form-group">
              <input type="email" ref="email" name="email" className="form-control required" placeholder="Username" required />
            </div>
            <div className="form-group">
              <input type="password" ref="pass" name="password" className="form-control required" placeholder="Password" required />
            </div>
            <button type="submit" onClick= {this.handleSubmit} className="btn btn-primary block full-width m-b" disabled={isProcessing}>Login</button>
          </div>
        </form>
      </div>
    );
  }
});

module.exports = Login;
