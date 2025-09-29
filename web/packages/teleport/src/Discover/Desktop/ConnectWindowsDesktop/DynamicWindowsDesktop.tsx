import { H2, H3 } from 'design/Text';
import { useState } from 'react';
import Validation, { Validator } from 'shared/components/Validation';
import { LabelsCreater, StyledBox, TextIcon } from 'teleport/Discover/Shared';
import { ResourceLabel } from 'teleport/services/agents';
import * as Icons from 'design/Icon';
import Box from 'design/Box';
import { ButtonSecondary } from 'design/Button';
import FieldInput from 'shared/components/FieldInput';
import { requiredField, requiredPort } from 'shared/components/Validation/rules';
import Flex from 'design/Flex';
import { Indicator } from 'design/Indicator';

export interface DesktopInstance {
  id: string;
  desktopUri: string;
  desktopPort: string;
  labels: ResourceLabel[];
  connectionState: ConnectionState;
}

export interface DynamicWindowsDesktopProps {
  desktopInstance: DesktopInstance;
  onUpdate(id: string, updates: Partial<DesktopInstance>): void;
  onRemove(id: string): void;
}

export type ConnectionState = 'idle' | 'connecting' | 'connected' | 'failed';

export function DynamicWindowsDesktop({
  desktopInstance,
  onUpdate,
  onRemove,
}: DynamicWindowsDesktopProps) {
  const { id, desktopUri, desktopPort, labels, connectionState } = desktopInstance;

  const [showConnection, setShowConnection] = useState(false);
  const [editing, setEditing] = useState(true);

  const nextLabelTxt = labels.length
    ? 'Finish Adding Labels'
    : 'Skip Adding Labels';

  let connectionTxt: string;
  switch (connectionState) {
    case 'connecting':
      connectionTxt = 'Connecting';
      break;
    case 'connected':
      connectionTxt = 'Connected';
      break;
    case 'failed':
      connectionTxt = 'Failed to connect';
      break;
    case 'idle':
    default:
      connectionTxt = '';
      break;
  }

  function updateLabels(newLabels: ResourceLabel[]) {
    onUpdate(id, { labels: newLabels });
  }

  function updateDesktopUri(uri: string) {
    onUpdate(id, { desktopUri: uri });
  }

  function updateDesktopPort(port: string) {
    onUpdate(id, { desktopPort: port });
  }

  function toggleShowConnection(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    if (!showConnection) {
      setShowConnection(!showConnection);
      validator.reset();
      return;
    }
  }

  function addDesktop(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    onUpdate(id, { connectionState: 'connecting' });
    setEditing(false);

    // TEST
    window.setTimeout(() => {
      if (Math.random() > 0.5) {
        onUpdate(id, { connectionState: 'connected' });
      } else {
        onUpdate(id, { connectionState: 'failed' });
      }
    }, 2000);
  }

  function toggleEdit() {
    if (desktopUri && desktopPort) {
      setEditing(!editing);
    }
    // setShowConnection(!showConnection);
    // onUpdate(id, { connectionState: 'idle' });
  }

  function StatusIcon() {
    switch (connectionState) {
      case 'connecting':
        return <Indicator delay='none' size={24}/>
      case 'connected':
        return <Icons.Check color='success.main'/>
      case 'failed':
        return <Icons.Warning color='warning.main'/>
      case 'idle':
      default:
        return <></>
    }
  }

  return (
    <StyledBox>
      <Flex flexDirection='row'>
        <Box mr={2}>
          <Icons.ChevronDown
            onClick={toggleEdit}
            style={{
              transform: editing ? 'rotate(0deg)' : 'rotate(-90deg)',
              transition: 'transform 0.2s ease'
            }}
          />
        </Box>
        <Box flex={1}>
          {editing && (
            <>
              <header>
                <H2>Add a Dynamic Windows Desktop resource</H2>
              </header>
              <Validation>
                {({ validator }) => (
                  <>
                    <LabelsCreater
                      labels={labels}
                      setLabels={updateLabels}
                      isLabelOptional={true}
                      noDuplicateKey={true}
                      disableBtns={showConnection}
                    />
                    <Box mt={3}>
                      <ButtonSecondary
                        width='200px'
                        type='submit'
                        onClick={() => toggleShowConnection(validator)}
                      >
                        {showConnection ? 'Edit Labels' : nextLabelTxt}
                      </ButtonSecondary>
                    </Box>
                    {showConnection && (
                      <>
                        <Flex mt={3} width='30rem'>
                          <FieldInput
                            autoFocus
                            label='Desktop Endpoint'
                            rule={requiredField('desktop endpoint is required')}
                            value={desktopUri}
                            placeholder='desktop.example.com'
                            onChange={e => updateDesktopUri(e.target.value)}
                            width='70%'
                            mr={2}
                            toolTipContent='Desktop location and connection information.'
                          />
                          <FieldInput
                            label='Endpoint Port'
                            rule={requiredPort}
                            value={desktopPort}
                            placeholder='3389'
                            onChange={e => updateDesktopPort(e.target.value)}
                            width='30%'
                          />
                        </Flex>
                        <Box mt={3}>
                          <ButtonSecondary
                            width='200px'
                            type='submit'
                            onClick={() => addDesktop(validator)}
                          >
                            Add Desktop
                          </ButtonSecondary>
                        </Box>
                      </>
                    )}
                  </>
                )}
              </Validation>
            </>
          )}
          {!editing && (
            <Flex flexDirection='row' width='100%'>
              <H2>{connectionTxt} to {desktopUri}:{desktopPort}</H2>
              <Box ml='auto'>
                <StatusIcon />
              </Box>
            </Flex>
          )}
        </Box>
      </Flex>
    </StyledBox>
  );
}