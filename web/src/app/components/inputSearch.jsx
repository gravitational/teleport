var React = require('react');
var {debounce} = require('_');

var InputSearch = React.createClass({

  getInitialState(){
    this.debouncedNotify = debounce(()=>{
        console.log('dada');
        this.props.onChange(this.state.value);
    }, 200);

    return {value: this.props.value};
  },

  onChange(e){
    this.setState({value: e.target.value});
    this.debouncedNotify();
  },

  componentDidMount() {
  },

  componentWillUnmount() {
  },

  render: function() {
    return (
      <div className="grv-search">
        <input placeholder="Search..." className="form-control input-sm"
          value={this.state.value}
          onChange={this.onChange} />
      </div>
    );
  }
});

module.exports = InputSearch;
