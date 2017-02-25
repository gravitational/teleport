import React from 'react';

class Indicator extends React.Component {

  constructor(props) {
    super(props);    
  }
  
  render() {    
    let { enabled = true, type } = this.props;

    if (!enabled) {
      return null;
    }

    if (type === 'bounce') {
      return <ThreeBounce />
    }

    if (type === 'circle') {
      return <Circle />
    }
  }
}

const ThreeBounce = () => (
  <div className="grv-spinner sk-spinner sk-spinner-three-bounce">
    <div className="sk-bounce1"/>
    <div className="sk-bounce2"/>
    <div className="sk-bounce3"/>
  </div>
)
  
const Circle = () => (
  <div className="sk-spinner sk-spinner-circle">
    <div className="sk-circle1 sk-circle"></div>
    <div className="sk-circle2 sk-circle"></div>
    <div className="sk-circle3 sk-circle"></div>
    <div className="sk-circle4 sk-circle"></div>
    <div className="sk-circle5 sk-circle"></div>
    <div className="sk-circle6 sk-circle"></div>
    <div className="sk-circle7 sk-circle"></div>
    <div className="sk-circle8 sk-circle"></div>
    <div className="sk-circle9 sk-circle"></div>
    <div className="sk-circle10 sk-circle"></div>
    <div className="sk-circle11 sk-circle"></div>
    <div className="sk-circle12 sk-circle"></div>
  </div>
)
  
export default Indicator;