import React from 'react';
import Layout from 'app/components/common/layout';
import Separator from 'app/components/common/separator';

const DESC = 'Users with this permission will be able to create and modify Roles and Authentication settings.';


const RoleDetailsAdmin = React.createClass({              
  render() {                                    
    let { enabled, onChange, editable } = this.props;        
    return (                                        
      <div className="m-b-lg">
        <Layout.Flex dir="column" justify="between" style={{ flex: "1" }} align="center">
          <h3 className="no-margins"> Administration </h3>
          <div>
          </div>
        </Layout.Flex>
        <Separator />
        <div className="m-l">          
          <div className="checkbox">
            <label>
              <input
                type="checkbox"
                disabled={!editable}
                onChange={() => onChange(!enabled)}            
                checked={enabled} />
              Allow Changing Permissions and Auth Settings
            </label>
            <div className="help-block m-l-md">
              {DESC}
            </div>
          </div>          
        </div>        
      </div>                                            
    );
  }
});

export default RoleDetailsAdmin;