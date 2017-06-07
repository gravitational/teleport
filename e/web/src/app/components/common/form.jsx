import React from 'react';

export const Separator = () => (<div className="grv-line-solid m-t-lg m-b-lg"></div>);

const Form = React.createClass({

  refCb(e) {    
    if (this.props.refCb) {
      this.props.refCb(e);
    }
  },

  onSubmit(e) {    
    e.preventDefault();
    return false;
  },

  render() {    
    let { className='', style = {}, children } = this.props;
    return (
      <form ref={this.refCb}
        className={className}
        style={style}
        onSubmit={this.onSubmit}>
        {children}
      </form>
    );
  }
});

export default Form;