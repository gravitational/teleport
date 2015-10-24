'use strict';

var SitesPage = React.createClass({
  getInitialState: function(){
      return {sites: []};
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
        this.setState({sites: data});
      }.bind(this),
      error: function(xhr, status, err) {
        console.error(this.props.url, status, err.toString());
      }.bind(this)
    });
  },
  render: function() {
    return (
<div id="wrapper">
   <LeftNavBar current="sites"/>
   <div id="page-wrapper" className="gray-bg">
       <TopNavBar/>
       <PageHeader title="Sites" icon="fa fa-cloud" breadcrumbs={[{url: "/web/sites", title: "Sites"}]}/>
            <div className="wrapper wrapper-content animated fadeInRight">
            <Box>
                <SitesBox sites={this.state.sites}/>
            </Box>
       </div>
       <PageFooter/>
   </div>
</div>
    );
  }
});

var SitesBox = React.createClass({
    render: function() {
        if (this.props.sites.length == 0) {
            return (
<div className="text-center m-t-lg">
   <h1>
      Connected Sites
   </h1>     
   <small>There are no remote sites that registered. This may be caused by network disruption on your end.</small><br/><br/>
</div>);
        }
        return (
<div>
   <SitesTable {...this.props}/>
</div>
);
    }
})


var SitesTable = React.createClass({
  render: function() {
    var rows = this.props.sites.map(function (site, index) {
      return (
        <SiteRow site={site} key={index}/>
      );
    });
    return (
<table className="table table-striped">
   <thead>
      <tr>
        <th>Name</th>
        <th>Status</th>            
        <th>Last Connected</th>
        <th>Action</th>
      </tr>
   </thead>
   <tbody>
      {rows}
   </tbody>
</table>);
  }
});


var SiteRow = React.createClass({
    render: function() {
        var connected = new Date(this.props.site.last_connected);
        var statusClass = this.props.site.status == "online"? "label label-primary": "label";
        var siteURL = "/tun/"+this.props.site.name+"/";
    return (
<tr>
  <td><a href={siteURL}>{this.props.site.name}</a></td>
  <td><span className={statusClass}>{this.props.site.status}</span></td>
  <td>{timeSince(connected)} ago </td>
  <td><a href={siteURL}><i className="fa fa-keyboard-o text-navy"></i></a></td>
</tr>
    );
  }
});

React.render(
  <SitesPage url="/api/sites" pollInterval={2000}/>,
  document.body
);
