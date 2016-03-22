/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

var React = require('react');
var {debounce} = require('_');

var InputSearch = React.createClass({

  getInitialState(){
    this.debouncedNotify = debounce(()=>{        
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
