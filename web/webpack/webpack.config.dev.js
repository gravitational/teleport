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

var baseCfg = require('./webpack.base');

var output = Object.assign({}, baseCfg.output, {
  filename: '[name].js'
});

var cfg = {
  entry: baseCfg.entry,
  resolve: baseCfg.resolve,
  output: output,    
  cache: true,

  //devtool: 'source-map',
  
  module: {
    loaders: [
      baseCfg.loaders.fonts,
      baseCfg.loaders.svg,
      baseCfg.loaders.images,
      baseCfg.loaders.js({withHot: true}),
      baseCfg.loaders.scss
    ]
  },

  plugins:  [    
    baseCfg.plugins.devBuild,
    baseCfg.plugins.hotReplacement,
    baseCfg.plugins.createIndexHtml,
    baseCfg.plugins.vendorBundle
  ]
  
};

module.exports = cfg;
