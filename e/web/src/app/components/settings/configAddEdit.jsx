import React, { PropTypes } from 'react';
import classnames from 'classnames';
import * as Alerts from 'telebase-app/components/alerts';
import Layout from '../common/layout';
import Button from '../common/button';
import YamlEditor from './yamlEditor';
import withChangeTracker from './../withChangeTracker';

class AddEditConfig extends React.Component {

  static propTypes = {
    changeTracker: PropTypes.object.isRequired,
    saveAttempt: PropTypes.object.isRequired,
    access: PropTypes.object.isRequired,
    item: PropTypes.object.isRequired
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
    this.init(props);
  }

  init(props){
    this.isNew = props.item.isNew;
    this.original = props.item.getContent();
    this.current = this.original;
    this.state.isDirty = false;
  }

  componentWillUnmount() {
    this.props.changeTracker.unregister(this);
  }

  componentDidMount() {
    this.props.changeTracker.register(this);
  }

  componentWillReceiveProps(nextProps) {
    if(nextProps.item !== this.props.item){
      this.init(nextProps)
    }
  }

  hasChanges(){
    return this.state.isDirty;
  }

  renderFooter() {
    const { saveAttempt, access } = this.props;
    const { isProcessing } = saveAttempt;
    const isSecondaryBtnEnabled = this.isNew || access.remove || isProcessing;
    const secondaryBtnCb = this.isNew ? this.props.onCancel : this.props.onDelete;
    const secondaryBtnText = this.isNew ? 'Cancel' : 'Delete';
    const secondaryBtnClassName = classnames({
      'btn-danger': !this.isNew,
      'btn-default': this.isNew
    });

    let isPrimaryBtnEnabled = true;
    if(this.isNew){
      isPrimaryBtnEnabled = access.create;
    }else{
      isPrimaryBtnEnabled = this.state.isDirty;
    }

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
          isDisabled={!isSecondaryBtnEnabled}
          className={secondaryBtnClassName}
          onClick={secondaryBtnCb}>
          {secondaryBtnText}
        </Button>
      </div>
    )
  }

  render(){
    const { item, saveAttempt, access } = this.props;
    if(!item){
      return null;
    }

    const id = item.getId();
    const readOnly = !this.isNew && !access.edit;
    const { isFailed, message } = saveAttempt;
    const $footer = this.renderFooter();
    return (
      <Layout.Flex style={{flex: "1"}} className="grv-settings-res-editor">
        <div className="full-width">
          { isFailed && <Alerts.Danger className="m-b-sm"> {message} </Alerts.Danger> }
          { readOnly && <Alerts.Info className="m-b-sm"> You do not have permissions to edit this resource </Alerts.Info> }
        </div>
        <YamlEditor
          key={id}
          readOnly={readOnly}
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

