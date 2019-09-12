import React from 'react'
import PropTypes from 'prop-types';
import styled from 'styled-components';
import { Box, ButtonSecondary, ButtonPrimary, LabelInput } from 'design';
import * as Alerts from 'design/Alert';
import { useAttempt, withState } from 'shared/hooks';
import Dialog, { DialogHeader, DialogTitle, DialogContent, DialogFooter} from 'design/DialogConfirmation';
import { updateLicense } from 'e-gravity/cluster/flux/actions';

export function UpdateLicenseDialog(props){
  const { onClose, onUpdate, attempt, attemptActions } = props;

  const refTextArea = React.useRef(null);

  const onOk = () => {
    attemptActions
      .do(() => onUpdate(refTextArea.current.value))
      .then(() => onClose());
  };

  const isDisabled = attempt.isProcessing;

  return (
    <Dialog
      disableEscapeKeyDown={isDisabled}
      onClose={onClose}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>
          UPDATE YOUR GRAVITY LICENSE
        </DialogTitle>
      </DialogHeader>
      <DialogContent minHeight="300px">
        {attempt.isFailed && (
          <Alerts.Danger children={attempt.message} />
        )}
        <LabelInput>INSERT YOUR NEW LICENSE BELOW</LabelInput>
        <StyledLicense flex="1" as="textarea" autoComplete="off" p="2"
          ref={refTextArea}
          color="primary.contrastText"
          bg="bgTerminal"
          type="text"
          placeholder="Insert new license here"/>
      </DialogContent>
      <DialogFooter>
        <ButtonPrimary mr="3" disabled={isDisabled} onClick={onOk}>
          Update License
        </ButtonPrimary>
        <ButtonSecondary disabled={isDisabled} onClick={onClose}>
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

UpdateLicenseDialog.propTypes = {
  onClose: PropTypes.func.isRequired,
  onUpdate: PropTypes.func.isRequired,
  attempt: PropTypes.object.isRequired,
  attemptActions: PropTypes.object.isRequired,
}

const StyledLicense = styled(Box)`
  font-family: ${ props => props.theme.fonts.mono};
  border: none;
  min-width: 600px;
  max-width: 800px;
  word-break: break-all;
  word-wrap: break-word;
`

function mapState(){
  const [ attempt, attemptActions ] = useAttempt();
  return {
    onUpdate: updateLicense,
    attempt,
    attemptActions,
  }
}

export default withState(mapState)(UpdateLicenseDialog);