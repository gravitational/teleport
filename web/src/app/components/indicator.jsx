import React from 'react';

const WHEN_TO_DISPLAY = 100; // 0.2s;

class Indicator extends React.Component {

  constructor(props) {
    super(props);    
    this._timer = null;
    this.state = {
      canDisplay: false
    }
  }

  componentDidMount() {
    this._timer = setTimeout(() => {
      this.setState({
        canDisplay: true
      })
    }, WHEN_TO_DISPLAY);
  }
  
  componentWillUnmount() {
    clearTimeout(this._timer);
  }
  
  render() {    
    const { type = 'bounce' } = this.props;
    
    if (!this.state.canDisplay) {
      return null;
    }

    if (type === 'bounce') {
      return <ThreeBounce />
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
  
export default Indicator;