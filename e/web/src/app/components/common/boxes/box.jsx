import React from 'react';
import './box.scss'
import Layout from './../layout';
import Separator from './../separator';

let Box = React.createClass({

  renderBoxHeader() {
    let { title } = this.props;

    if (!title) {
      return null;
    }

    return (
      <BoxHeader>        
        <h3>{title}</h3>                
      </BoxHeader>
    )
  },

  render() {
    let { className, style} = this.props;
    let containerClass = 'grv-box-container';

    if (className) {
      containerClass = `${containerClass} ${className}`;
    }

    let $header = this.renderBoxHeader();
    let boxStyle = { ...style };

    return (
      <div style={boxStyle} className={containerClass}>
        {$header}
        <div className="grv-box-content">
          {this.props.children}
        </div>
      </div>
    );
  }
});

let BoxHeader = props => {
  return (
    <div className="grv-box-header">
      <Layout.Flex className="" dir="column" align="center">
        {props.children}
      </Layout.Flex>
      <Separator />
    </div>
  )
}
  
Box.Header = BoxHeader

export default Box;
