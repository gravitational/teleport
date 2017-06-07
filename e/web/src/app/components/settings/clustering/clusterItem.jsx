import React from 'react';
import 'brace/mode/json';
import Separator from 'app/components/common/separator';
import Button from 'app/components/common/button';
import Layout from 'app/components/common/layout';
import Form from 'app/components/common/form';
import actions from './../../../flux/settingsClusters/actions';

const Label = ({text}) => ( 
  <label style={{ width: "130px" }} className="text-bold m-t-xs"> {text} </label>
)

const inputSmallStyleMaxWidth = {
  maxWidth: "400px"
}

const inputSmallStyle = {
  flex: "1",  
  ...inputSmallStyleMaxWidth
}

const RoleLabel = ({ name }) => (
  <label title={name} className="grv-settings-cluster-role-label m-r-xs">
    {name}
  </label>
)

const Value = ({text}) => {
  return text ? <div>{text}</div> : <small className="text-muted">Empty</small>  
}
  
const ClusterItem = React.createClass({

  getInitialState() {
    let { cluster } = this.props;
    this.snapshot = JSON.stringify(cluster)
    this.isDirty = false;
    return {
      ...cluster
    }
  },

  componentDidMount() {
    this.context.register(this);
  },
  
  componentWillUnmount() {
    this.context.unregister(this);
  },

  onChangeConnected(e) {
    let enabled = e.target.value === 'true';
    this.setState({
      enabled
    })
  },

  handleIsDirty() {
    this.isDirty = JSON.stringify(this.state) !== this.snapshot;
  },

  hasChanges() {
    return this.isDirty;
  },

  onSave() {    
    this.setState({
      isSaving: true
    })
          
    actions.saveCluster(this.state)
      .fail(() => {
        this.setState({
          isSaving: false
        });
      });            
  },
              
  render() {                  
    this.handleIsDirty();    
    let { enabled, isSaving, proxyAddress, reverseTunnelAddress, name, roles } = this.state;
    let isSaveEnabled = this.hasChanges();    
    let $roles = roles.map((r, key) => <RoleLabel key={key} name={r}/> )

    if ($roles.length === 0) {
      $roles = <Value/>
    }
    
    return (      
      <Form refCb={e => this.refForm = e} className="grv-settings-auth-connector-item m-b-md">        
        <Separator size="xs"/>
        <div className="p-w-xs m-b">        
          <Layout.Flex dir="row" align="center" className="m-t-md">
            <Label text="Enabled:" />                                
            <div style={{ flex: "1" }}>
              <label className="radio-inline">
                <input type="radio" className="inlineRadioOptions"
                  value={true}  
                  onChange={this.onChangeConnected}
                  checked={enabled}
                  /> Yes
              </label>
              <label className="radio-inline">
                <input type="radio" className="inlineRadioOptions"
                  value={false}  
                  checked={!enabled}
                  onChange={this.onChangeConnected}                  
                  /> No
              </label>                            
            </div>                                                                    
          </Layout.Flex>                              
          <Layout.Flex dir="row" align="center" className="m-t-xs">
            <Label text="Name:" />                                
            <div style={{flex: "1"}}>
              <div                
                style={inputSmallStyle}>                             
                <Value text={name}/>
              </div>
            </div>                                                                    
          </Layout.Flex>                    
          <Layout.Flex dir="row" align="center" className="m-t-xs">
            <Label text="Roles:" />                                
            <div style={{flex: "1"}}>
              <div                
                style={inputSmallStyle}>             
                {$roles}
              </div>
            </div>                                                                    
          </Layout.Flex>                    
          <Layout.Flex dir="row" align="center" className="m-t-xs">
            <Label text="Proxy:" />                                
            <div style={{flex: "1"}}>
              <div                
                style={inputSmallStyle}>                             
                <Value text={proxyAddress}/>
              </div>
            </div>                                                                    
          </Layout.Flex>                              
          <Layout.Flex dir="row" align="center" className="m-t-xs">
            <Label text="Reverse Tunnel:" />                                
            <div style={{flex: "1"}}>
              <div                
                style={inputSmallStyle}>                             
                <Value text={reverseTunnelAddress}/>
              </div>
            </div>                                                                    
          </Layout.Flex>                              
        </div>         
        <div className="m-t-sm">
          <Button className="btn-primary m-r-sm"
            isDisabled={!isSaveEnabled}            
            isProcessing={isSaving}
            onClick={this.onSave}>            
            Save
          </Button>          
        </div>                 
      </Form>
    );
  }
});

ClusterItem.contextTypes = {
  register: React.PropTypes.func.isRequired,
  unregister: React.PropTypes.func.isRequired
};

export default ClusterItem;
