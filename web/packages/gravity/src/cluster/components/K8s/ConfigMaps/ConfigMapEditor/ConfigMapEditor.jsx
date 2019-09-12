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
import PropTypes from 'prop-types';
import { endsWith } from 'lodash';
import * as Alerts from 'design/Alert';
import Dialog, {
  DialogContent,
  DialogHeader,
} from 'design/Dialog';
import {
  Flex,
  ButtonPrimary,
  ButtonSecondary,
  Text,
} from 'design';
import TextEditor from 'shared/components/TextEditor';
import Attempt from './Attempt';
import Tabs from './Tabs';

class ConfigMapEditor extends React.Component {
  state = {
    activeTabIndex: 0,
    dirtyTabs: [],
  };

  onChangeTab = activeTabIndex => {
    this.setState({ activeTabIndex });
  };

  makeTabDirty = isDirty => {
    const { dirtyTabs, activeTabIndex } = this.state;
    dirtyTabs[activeTabIndex] = isDirty;
    this.setState({
      dirtyTabs,
    });
  };

  onSave = attemptActions => {
    const { data } = this.props.configMap;
    const editorData = this.yamlEditorRef.getData();
    const changes = {};

    // apply editor changes
    data.forEach((item, index) => {
      changes[item.name] = editorData[index];
    });

    attemptActions
      .do(() => this.props.onSave(changes))
      .then(this.props.onClose);
  };

  render() {
    const { onClose, configMap } = this.props;
    const { data = [], id, name, namespace } = configMap;
    const { activeTabIndex, dirtyTabs } = this.state;
    const disabledSave = !dirtyTabs.some(t => t === true);
    const textEditorData = makeTextEditorData(data);

    return (
      <Dialog
        dialogCss={dialogCss}
        disableEscapeKeyDown={false}
        onClose={onClose}
        open={true}
      >
        <Attempt onRun={this.onSave}>
          {({ attempt, run }) => (
            <>
              <DialogHeader justifyContent="space-between">
                <Text typography="h3">{name}</Text>
                <Text as="span">NAMESPACE: {namespace}</Text>
              </DialogHeader>
              {attempt.isFailed && (
                <Alerts.Danger mb="4">{attempt.message}</Alerts.Danger>
              )}
              <DialogContent>
                <Tabs
                  items={data}
                  onSelect={this.onChangeTab}
                  activeTab={activeTabIndex}
                  dirtyTabs={dirtyTabs}
                />
                <Flex flex="1">
                  <TextEditor
                    ref={e => (this.yamlEditorRef = e)}
                    id={id}
                    onDirty={this.makeTabDirty}
                    data={textEditorData}
                    activeIndex={activeTabIndex}
                  />
                </Flex>
              </DialogContent>
              <div>
                <ButtonPrimary
                  onClick={run}
                  disabled={disabledSave || attempt.isProcessing}
                  mr="3"
                >
                  Save Changes
                </ButtonPrimary>
                <ButtonSecondary
                  disabled={attempt.isProcessing}
                  onClick={onClose}
                >
                  CANCEL
                </ButtonSecondary>
              </div>
            </>
          )}
        </Attempt>
      </Dialog>
    );
  }
}

ConfigMapEditor.propTypes = {
  configMap: PropTypes.object.isRequired,
  onSave: PropTypes.func.isRequired,
  onClose: PropTypes.func.isRequired,
};

function makeTextEditorData(configMapData) {
  return configMapData.map(item => ({
    content: item.content,
    type: getType(item.name),
  }));
}

function getType(name) {
  if (endsWith(name, 'json')) {
    return 'json';
  }

  return 'yaml';
}

const dialogCss = () => `
  height: 80%
  width: calc(100% - 20%);
  max-width: 1400px;
`;

export default ConfigMapEditor;
