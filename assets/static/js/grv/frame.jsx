'use strict';

var UserBarItem = React.createClass({
    render: function() {
        return (
            <li className="nav-header">
              <div className="dropdown profile-element">
                <a data-toggle="dropdown" className="dropdown-toggle" href="#">
                  <span className="clear">
                    <span className="block m-t-xs">
                      <strong className="font-bold">David Williams</strong>
                    </span>
                    <span className="text-muted text-xs block">
                      Engineer <b className="caret"></b></span>
                  </span>
                </a>
                <ul className="dropdown-menu animated fadeInRight m-t-xs">
                  <li><a href="/logout">Logout</a></li>
                </ul>
              </div>
              <div className="logo-element">
                Gravitational Teleport
              </div>
            </li>
        );
    }
});

var LeftNavBar = React.createClass({
    className: function(item){
        if (item == this.props.current) {
            return "active";
        }
        return "";
    },
    render: function(){
        return (
<nav className="navbar-default navbar-static-side" role="navigation">
    <div className="sidebar-collapse">
        <ul className="nav" id="side-menu">
            <UserBarItem/>
            <li className={this.className("keys")}>
                <a href="/keys">
                    <i className="fa fa-laptop"></i>
                    <span className="nav-label">Keys</span>
                </a>
            </li>
            <li className={this.className("events")}>
                <a href="/events">
                    <i className="fa fa-list"></i>
                    <span className="nav-label">Timeline</span>
                </a>
            </li>
            <li className={this.className("webtuns")}>
                <a href="/webtuns">
                    <i className="fa fa-arrows-h"></i>
                    <span className="nav-label">Web Tunnels</span>
                </a>
            </li>
            <li className={this.className("servers")}>
                <a href="/servers">
                    <i className="fa fa-hdd-o"></i>
                    <span className="nav-label">Servers</span>
                </a>
            </li>
            <li className={this.className("sessions")}>
                <a href="/sessions">
                    <i className="fa fa-wechat"></i>
                    <span className="nav-label">Sessions</span>
                </a>
            </li>
        </ul>
    </div>
</nav>
);
  }
});


var TopNavBar = React.createClass({
    render: function(){
        return (
<div className="row border-bottom">
   <nav className="navbar navbar-static-top" role="navigation" style={{marginBottom: 0}}>
      <div className="navbar-header">
         <a className="navbar-minimalize minimalize-styl-2 btn btn-primary " href="#"><i className="fa fa-bars"></i></a>
         <form role="search" className="navbar-form-custom" method="post" action="search_results.html">
            <div className="form-group">
               <input placeholder="Search for something..." className="form-control" name="top-search" id="top-search" type="text"/>
            </div>
         </form>
      </div>
      <ul className="nav navbar-top-links navbar-right">
         <li>
            <span className="m-r-sm text-muted welcome-message">Welcome to Gravitational Teleport!</span>
         </li>
         <li>
            <a href="/logout"><i className="fa fa-sign-out"></i> Log out</a>
         </li>
      </ul>
   </nav>
</div>);
   }
    
});

var PageHeader = React.createClass({
    render: function() {
        return (
<div className="row wrapper border-bottom white-bg page-heading">
   <div className="col-lg-10">
      <h2>{this.props.title}</h2>
      <ol className="breadcrumb">
         <li className="active">
            <a href={this.props.url}>{this.props.title}</a>
         </li>
      </ol>
   </div>
   <div className="col-lg-2">
   </div>
</div>);
    }
});

var PageFooter = React.createClass({
    render: function() {
        return (
<div className="footer">
    <div className="pull-right">
        You are logged in into <strong>gravitational.io</strong> teleport
    </div>
    <div>
        <strong>Copyright</strong> Gravitational &copy; 2015
    </div>
</div>);
    }
});


var Box = React.createClass({
    render: function() {
        var colClass = this.props.colClass || "col-lg-12";
        return (
            <div className="row">
              <div className={colClass}>
                <div className="ibox float-e-margins">
                  <div className="ibox-title">
                    <h5>{this.props.title}</h5>
                    <div className="ibox-tools">
                      <a className="collapse-link fa fa-chevron-up" style={{fontStyle:"italic"}}></a> 
                      <a className="dropdown-toggle fa fa-wrench" data-toggle="dropdown" href="#" style={{fontStyle:"italic"}}></a>
                      <ul className="dropdown-menu dropdown-user">
                        <li>
                          <a href="#">Config option 1</a>
                        </li>
                      </ul>
                      <a className="close-link fa fa-times" style={{fontStyle:"italic"}}></a>
                    </div>
                  </div>
                  <div className="ibox-content">
                    <div className="table-responsive">
                      {this.props.children}
                    </div>
                  </div>
                </div>
              </div>
            </div>
        );
    }
});
