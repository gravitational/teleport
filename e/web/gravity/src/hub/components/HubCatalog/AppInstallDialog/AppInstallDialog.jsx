import React from 'react'
import PropTypes from 'prop-types';
import CmdText from 'gravity/components/CmdText';
import { ButtonSecondary, Text } from 'design';
import { AppKindEnum } from 'gravity/services/applications';
import Dialog, { DialogHeader, DialogTitle, DialogContent, DialogFooter} from 'design/DialogConfirmation';

export default function AppInstallDialog(props){
  const { onClose, app } = props;
  const $content = app.kind === AppKindEnum.APP ?
    renderAppImage(app): renderClusterImage(app);

  const { name } = app;

  return (
    <Dialog onClose={onClose} open={true} dialogCss={dialogCss}>
      <DialogHeader>
        <DialogTitle>
          INSTALL {name}
        </DialogTitle>
      </DialogHeader>
      {$content}
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>
          Close
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

AppInstallDialog.propTypes = {
  onClose: PropTypes.func.isRequired,
  app:  PropTypes.object.isRequired,
}

function renderAppImage(app){
  const { name, version } = app;
  const cmd = `$ gravity app install ${name}-${version}.tar`;
  return (
    <DialogContent>
      <Text typography="paragraph" color="primary.contrastText" mb="4">
        This image contains <ImageName name={name}/> but does not include the Kubernetes runtime required to run it.
        To install <ImageName name={name}/>, you must download it to an existing Gravity cluster node and execute the following command:
      </Text>
      <Instructions cmd={cmd} mb="4"/>
      <Text typography="paragraph" color="primary.contrastText">
        Follow the CLI for post-install instructions.
      </Text>
    </DialogContent>
  )
}

function renderClusterImage(app){
  const { name, version } = app;
  const cmd = `$ tar -xf ${name}-${version}.tar\n$ ./gravity install`;
  return (
    <DialogContent>
      <Text typography="paragraph" color="primary.contrastText" mb="4">
        This image contains <ImageName name={name}/>. It includes the Kubernetes
        runtime and is ready to be deployed. To install this image,
        you must download it to a target Linux machine and execute the following CLI commands:
      </Text>
      <Instructions cmd={cmd} mb="4"/>
      <Text typography="paragraph" color="primary.contrastText">
        Follow the CLI for post-install instructions.
      </Text>
    </DialogContent>
  )
}

function ImageName({name}){
  return <Text as="span" bold typography="h5">{name}</Text>
}

function Instructions({cmd, ...rest}){
  return (
    <CmdText cmd={cmd} style={{ whiteSpace: "pre-line" }} {...rest} />
  )
}

const dialogCss = () => `
  max-width: 600px;
`

export {
  AppKindEnum
}