/*
Copyright 2019 Gravitational, Inc.

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

/*eslint no-console: "off"*/

import moment from 'moment';
import cfg from '../config';

export function displayK8sAge(created){
  const now = moment(new Date());
  const end = moment(created);
  const duration = moment.duration(now.diff(end));
  return duration.humanize();
}

export function displayDate(date) {
  try {
    return moment(date).format(cfg.dateFormat);
  } catch (err) {
    console.error(err);
    return 'undefined';
  }
}

export function displayDateTime(date) {
  try {
    return moment(date).format(cfg.dateTimeFormat);
  } catch (err) {
    console.error(err);
    return 'undefined';
  }
}