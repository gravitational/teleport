import React from 'react';
import classnames from 'classnames';
import Layout from 'app/components/common/layout';
import Button from 'app/components/common/button';
import YamlEditor from './yamlEditor';
import withChangeTracker from './../withChangeTracker';
import * as Alerts from 'app/components/common/alerts';

class AddEditConfig extends React.Component {
  
  static propTypes = {  
    changeTracker: React.PropTypes.object.isRequired,
    saveAttempt: React.PropTypes.object.isRequired
  }

  state = {
    isDirty: false
  }
      
  onSave = () => {    
    const item = this.props.item.setContent(this.current);    
    this.props.onSave(item)
  }
    
  onItemContentChange = yaml => {      
    const isDirty = yaml !== this.original;
    this.current = yaml;
    this.setState({ isDirty });
  }

  constructor(props){
    super(props);
    this.isNew = props.item.isNew;
    this.original = props.item.getContent();   
    this.current = this.current;    
  }
  
  componentWillUnmount() {
    this.props.changeTracker.unregister(this);
  }

  componentDidMount() {    
    this.props.changeTracker.register(this);
  }

  hasChanges(){
    return this.state.isDirty || this.isNew;
  }

  renderFooter(saveAttempt) {                
    const { isProcessing } = saveAttempt;    
    const isPrimaryBtnEnabled = this.isNew || this.state.isDirty;    
    const secondaryBtnCb = this.isNew ? this.props.onCancel : this.props.onDelete;
    const secondaryBtnText = this.isNew ? 'Cancel' : 'Delete';
    const secondaryBtnClassName = classnames({
      'btn-danger': !this.isNew,
      'btn-default': this.isNew
    });
              
    return (
      <div className="m-t">
        <Button size="sm"         
          onClick={this.onSave}
          isProcessing={isProcessing}
          isDisabled={!isPrimaryBtnEnabled}
          className="btn-primary m-r-sm">
          Save
        </Button>
        <Button size="sm"         
          className={secondaryBtnClassName}
          onClick={secondaryBtnCb}
          isDisabled={isProcessing}>            
          {secondaryBtnText}
        </Button>
      </div>     
    )
  }
  
  render(){
    const { item, saveAttempt } = this.props;                            
    if(!item){
      return null;
    }
    
    const { isFailed, message } = saveAttempt;
    const $footer = this.renderFooter(saveAttempt);    
    const className = classnames('grv-settings-res-editor', { 'm-l': !item.isNew });
    return (
      <Layout.Flex style={{flex: "1"}} className={className}>     
        <div className="full-width">          
          { isFailed && <Alerts.Danger className="m-b-sm"> {message} </Alerts.Danger> }
        </div>                       
        <YamlEditor
          key={item.key}
          data={item.content}
          onChange={this.onItemContentChange}
          name="rolemappings" required
          type="text"
        />                                   
        {$footer}             
      </Layout.Flex>
    )
  }
}

export default withChangeTracker(AddEditConfig);

