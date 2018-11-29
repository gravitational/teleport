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

import $ from 'jquery'
import expect from 'expect';
import ReactTestUtils from 'react-addons-test-utils';
import reactor from 'app/reactor';
import session from 'app/services/session';
import api from 'app/services/api';
import cfg from 'app/config';
import 'app/flux';

const spyOn = expect.spyOn;
const Dfd = $.Deferred;

export {
  cfg,
  reactor,
  session,
  expect,
  $,
  ReactTestUtils,
  spyOn,
  Dfd,
  api
}