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
import { Flex, Box, ButtonPrimary, Input, LabelInput } from 'design';
import Tag from './Tag';

const ENTER_KEY = 13;

function ClusterTags({onChange}) {
  const [ value, setValue ] = React.useState('');
  const [ tags, setTags ] = React.useState({});

  // notify parent about the change
  React.useEffect(() => {
    onChange(tags)
  }, [tags])

  function onChangeValue(e){
    setValue(e.target.value)
  }

  function onAddTags(){
    if(value){
      const tagsToAdd = {};
      parseTags(value).forEach( t => {
        tagsToAdd[t.key] = t.value;
      })

      setTags({
        ...tags,
        ...tagsToAdd
      });

      setValue('');
    }
  }

  function onKeyDown(e){
    if(e.which === ENTER_KEY){
      onAddTags();
    }
  }

  function onDelete(key){
    delete tags[key]
    setTags({
      ...tags
    });
  }

  return (
    <Box>
      <LabelInput>
        Create cluster labels
      </LabelInput>
      <Flex mb="4">
        <Input
          mr="3"
          value={value}
          onKeyDown={onKeyDown}
          onChange={onChangeValue}
          autoComplete="off"
          placeholder="key:value, key:value, ..."
        />
        <ButtonPrimary onClick={onAddTags}>
          Create
        </ButtonPrimary>
      </Flex>
      <LabelList tags={tags} onDelete={onDelete}/>
    </Box>
  );
}

function LabelList({ tags, onDelete }){
  const $tags = Object.keys(tags).map(key => (
    <Tag mr="2" mb="2"key={key} name={key} value={tags[key]} onClick={ () => onDelete(key) }/>
  ));

  return (
    <Flex flexWrap="wrap">
      {$tags}
    </Flex>
  )
}

function parseTags(str){
  return str.split(',')
    .map(t => {
      let [ key, value ] = t.split(':');
      // remove spaces
      key = key ? key.trim() : key;
      value = value ? value.trim() : value;
      return {
        key,
        value
      }
    })
    .filter(tag => tag.value && tag.key);
}

export default ClusterTags;
