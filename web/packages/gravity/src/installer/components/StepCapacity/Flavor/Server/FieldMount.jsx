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

import React from 'react';
import FieldInput from 'shared/components/FieldInput';
import { capitalize } from 'lodash';
import cfg from 'gravity/config';

export default function FieldMount({
  defaultValue,
  name,
  onChange,
  ...styles
}) {
  const [value, setValue] = React.useState(defaultValue);

  // notify parent about current value
  React.useEffect(() => {
    onChange({ name, value });
  }, [value]);

  // pick up this field title from web config
  const title = React.useMemo(() => {
    const mountCfg = cfg.getAgentDeviceMount(name);
    const title = mountCfg.labelText || name;
    return capitalize(title);
  }, [name]);

  function onFieldChange(e) {
    setValue(e.target.value);
  }

  return (
    <FieldInput
      mb="3"
      {...styles}
      value={value}
      label={title}
      rule={required(`${title} is required`)}
      onChange={onFieldChange}
    />
  );
}

const required = message => value => () => {
  return {
    valid: !!value,
    message,
  };
};
