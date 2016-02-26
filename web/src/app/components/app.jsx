var React = require('react');
var NavLeftBar = require('./navLeftBar');
var cfg = require('app/config');

var App = React.createClass({
  render: function() {
    return (
      <div className="grv-tlpt">
        <NavLeftBar/>
        <div className="row">
          <nav className="" role="navigation" style={{ marginBottom: 0 }}>
            <ul className="nav navbar-top-links navbar-right">
              <li>
                <span className="m-r-sm text-muted welcome-message">
                  Welcome to Gravitational Portal
                </span>
              </li>
              <li>
                <a href={cfg.routes.logout}>
                  <i className="fa fa-sign-out"></i>
                  Log out
                </a>
              </li>
            </ul>
          </nav>
        </div>
        <div className="grv-page">
          {this.props.children}
        </div>
      </div>
    );
  }
})

module.exports = App;
