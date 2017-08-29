import React from 'react';
import ace from 'brace';
import 'brace/mode/yaml';
import 'brace/ext/searchbox';
//import 'brace/theme/iplastic';

const { UndoManager } = ace.acequire('ace/undomanager');

class YamlEditor extends React.Component{

  onChange = () => {    
    let value = this.session.getValue();        
    if (this.props.onChange) {
      this.props.onChange(value);
    }      
  }  

  componentDidUpdate(){
    this.editor.resize();
  }

  initEditSessions() {
    let { data } = this.props;
    let undoManager = new UndoManager();  
    data = data || '';      
    this.isDirty = false;
    this.session = new ace.EditSession(data);      
    this.session.setOptions({ tabSize: 2, useSoftTabs: true });
    this.session.setUndoManager(undoManager);
    this.session.setUseWrapMode(true)        
    this.session.setMode("ace/mode/yaml");  
    //this.editor.setTheme('ace/theme/iplastic');
    this.editor.setSession(this.session);
    this.editor.renderer.setShowGutter(true);
  }
  
  componentDidMount() {
    this.editor = ace.edit(this.refs.ace_viewer);    
    this.editor.renderer.setShowGutter(false);
    this.editor.renderer.setShowPrintMargin(false);
    this.editor.renderer.setOption('showLineNumbers', true)
    this.editor.setFadeFoldWidgets(true);
    this.editor.setWrapBehavioursEnabled(true);
    this.editor.setHighlightActiveLine(false);
    this.editor.setShowInvisibles(false);      
    this.editor.setReadOnly(false);    
    this.editor.on('input', this.onChange);
    this.initEditSessions(this.props.initialData);        
    this.editor.focus();    
  }

  componentWillUnmount() {
    this.editor.destroy();
    this.editor = null;
    this.session = null;    
  }
    
  shouldComponentUpdate() {
    return true;
  }

  render() {    
    return (      
      <div style={mainStyle}>
        <div className="grv-settings-json-editor">            
          <div ref="ace_viewer" style={editorStyle}></div>                  
        </div>                        
      </div>                  
    )
  }
}

const mainStyle = {    
  width: "100%",
  overflow: "auto",
  flex: "1",
  display: "flex",
  position: "relative",
  border: "1px solid #e7eaec",
  padding: "2px"
}

const editorStyle = {
  position: 'absolute',
  top: '0px',
  right: '0px',
  bottom: '0px',
  left: '0px'
};

export default YamlEditor;
