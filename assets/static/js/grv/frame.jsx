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
    items: function() {
        return grv.nav_sections.concat([
            {icon: "fa fa-key", url: grv.path("keys"), title: "Keys", key: "keys"},
            {icon: "fa fa-list", url: grv.path("events"), title: "Timeline", key: "events"},
            {icon: "fa fa-arrows-h", url: grv.path("webtuns"), title: "Web Tunnels", key: "webtuns"},
            {icon: "fa fa-hdd-o", url: grv.path("servers"), title: "Instances", key: "servers"},
            {icon: "fa fa-wechat", url: grv.path("sessions"), title: "Sessions", key: "sessions"},
        ]);
    },
    render: function(){
        var self = this;
        var items = this.items().map(function(i, index){
            return (<li className={self.className(i.key)}>
                <a href={i.url}>
                    <i className={i.icon}></i>
                    <span className="nav-label">{i.title}</span>
                </a>
            </li>);
        });
        return (
<nav className="navbar-default navbar-static-side" role="navigation">
    <div className="sidebar-collapse">
        <ul className="nav" id="side-menu">
            <UserBarItem/>
            {items}
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
        var brs = this.props.breadcrumbs || [];
        return (
            <div className="row wrapper border-bottom white-bg page-heading">
              <div className="col-lg-10">
                <h2><i className={this.props.icon}></i> {this.props.title}</h2>
                <ol className="breadcrumb">
                  {brs.map(function(b, idx) {
                      if (idx == brs.length -1 ) {
                          return <li className="active"><a href={b.url}><strong>{b.title}</strong></a></li>;
                      }
                      return <li><a href={b.url}>{b.title}</a></li>;
                   })}
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
                    {this.props.children}
                  </div>
                </div>
              </div>
            </div>
        );
    }
});
