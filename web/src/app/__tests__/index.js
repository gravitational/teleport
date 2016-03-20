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
module.exports.React = require('react');
module.exports.ReactDOM = require('react-dom');
module.exports.ReactTestUtils = require('react-addons-test-utils');
module.exports.expect = require('expect');
module.exports.$ = require('jQuery');
module.exports.Dfd = module.exports.$.Deferred;
module.exports.spyOn = module.exports.expect.spyOn;
module.exports.reactor = require('app/reactor');
module.exports.session = require('app/services/session');
module.exports.api = require('app/services/api');
module.exports.sampleData = require('./sampleData');
module.exports.cfg = require('app/config');
require('app/modules');
