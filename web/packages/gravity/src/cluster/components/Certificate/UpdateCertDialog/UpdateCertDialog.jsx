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
import styled from 'styled-components';
import PropTypes from 'prop-types';
import { withState, useAttempt } from 'shared/hooks';
import { Danger, Info } from 'design/Alert';
import Dialog, { DialogTitle, DialogFooter, DialogHeader, DialogContent } from 'design/Dialog';
import { Text, Input, ButtonPrimary, ButtonSecondary, Flex, Box } from 'design';
import { saveTlsCert } from 'gravity/cluster/flux/tlscert/actions';
import { withRouter } from 'react-router';

export class UpdateCertDialog extends React.Component {

  static propTypes = {
    onClose: PropTypes.func.isRequired,
    onSubmit: PropTypes.func.isRequired,
    attempt:  PropTypes.object.isRequired,
    attemptActions:  PropTypes.object.isRequired,
  }

  constructor(props) {
    super(props);
    this.refForm = null;
    this.state = {
      certFile: null,
      privateKeyFile: null,
      intermCertFile: null,
      showCertRequired: false,
      showPrivateKeyRequired: false,
    };

    this.refCert = React.createRef();
    this.refPrivateKey = React.createRef();
    this.refIntermCert = React.createRef();
  }

  validate(){
    const showCertRequired = this.state.certFile === null;
    const showPrivateKeyRequired = this.state.privateKeyFile === null;
    this.setState({
      showCertRequired,
      showPrivateKeyRequired
    })

    return !showCertRequired && !showPrivateKeyRequired;
  }

  onSubmit = e => {
    e.preventDefault();
    if(!this.validate()){
      return;
    }

    const { certFile, privateKeyFile, intermCertFile } = this.state;
    const { onSubmit, attemptActions } = this.props;
    attemptActions.do(() => {
      return onSubmit(
        certFile,
        privateKeyFile,
        intermCertFile
      );
    })
  }

  onCertFileSelected = e => {
    this.setState({
      certFile: e.target.files[0],
      showCertRequired: false
    });
  }

  onKeyFileSelected = e => {
    this.setState({
      privateKeyFile: e.target.files[0],
      showPrivateKeyRequired: false
    });
  }

  onIntermFileSelected = e => {
    this.setState({
      intermCertFile: e.target.files[0]
    });
  }

  onSelectCert = () => {
    this.refCert.current.value = null;
    this.refCert.current.click();
  }

  onSelectPrivateKey = () => {
    this.refPrivateKey.current.value = null;
    this.refPrivateKey.current.click();
  }

  onSelectIntermCert = () => {
    this.refIntermCert.current.value = null;
    this.refIntermCert.current.click();
  }

  render() {
    const {
      certFile,
      showCertRequired,
      showPrivateKeyRequired,
      intermCertFile,
      privateKeyFile } = this.state;

    const { onClose, attempt } = this.props;
    const { isFailed, isSuccess, isProcessing, message } = attempt;

    return (
      <Dialog onClose={onClose} open={true} disableEscapeKeyDown={isProcessing}>
        <DialogHeader>
          <DialogTitle>
            Update certificate
          </DialogTitle>
        </DialogHeader>
        <DialogContent width="600px">
          { isFailed && <Danger mb="4">{message}</Danger> }
          { isSuccess && <Info mb="4">Certificate has been updated</Info> }
          <Box mb="4">
            <Label text="Private Key" desc="must be in PEM format"/>
            <FileInput
              showRequired={showPrivateKeyRequired}
              fileName={privateKeyFile && privateKeyFile.name}
              onClick={this.onSelectPrivateKey} />
          </Box>
          <Box mb="4">
            <Label text="Certificate" desc="must be in PEM format"/>
            <FileInput
              showRequired={showCertRequired}
              fileName={certFile && certFile.name}
              onClick={this.onSelectCert} />
          </Box>
          <Box mb="4">
            <Label text="Intermediate Certificate" desc="optional"/>
            <FileInput showRequired={false}
              fileName={intermCertFile && intermCertFile.name}
              onClick={this.onSelectIntermCert} />
          </Box>
        </DialogContent>
        <DialogFooter>
          <ButtonPrimary mr="3"  disabled={isProcessing} onClick={this.onSubmit}>
            Update Certificate
          </ButtonPrimary>
          <ButtonSecondary onClick={this.props.onClose} disabled={isProcessing}>
            Cancel
          </ButtonSecondary>
        </DialogFooter>
        <HiddenInput ref={this.refCert} onChange={this.onCertFileSelected} />
        <HiddenInput ref={this.refPrivateKey} onChange={this.onKeyFileSelected} />
        <HiddenInput ref={this.refIntermCert} onChange={this.onIntermFileSelected} />
      </Dialog>
    );
  }
}

const Label = ({ text, desc = null }) => (
  <Flex mb="1" alignItems="center">
    <Text typography="h6" color="primary.contrastText"> {text} </Text>
    { desc  && <Text typography="body2" ml="1">({desc})</Text> }
  </Flex>
)

const FileInput = ({ fileName, name, onClick, showRequired=false }) => {
  return (
    <Flex flex="1">
      <Input
        readOnly
        hasError={showRequired}
        name={name}
        required
        value={fileName || ''}
      />
      <ButtonSecondary size="small" onClick={onClick}>
        Browse...
      </ButtonSecondary>
    </Flex>
  )};

const HiddenInput = styled.input.attrs({
  type: 'file',
  accept: '*.*',
  name: 'file'
})`
  display: none;
`

function mapState(){
  const [ attempt, attemptActions ] = useAttempt();
  return {
    attempt,
    attemptActions,
    onSubmit: saveTlsCert
  }
}

export default withRouter(withState(mapState)(UpdateCertDialog)) ;