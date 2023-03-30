/*
Copyright 2022 Gravitational, Inc.

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

import React, { useCallback, useEffect, useRef, useState } from 'react';
import styled from 'styled-components';

import { Box, Flex, Pill, Popover, Link, Text } from 'design';
import { Info, Warning } from 'design/Icon';

import { useClickOutside } from 'shared/hooks/useClickOutside';
import { useEscape } from 'shared/hooks/useEscape';

const VALID_LABEL = /^[a-z]+:\s?[0-9a-z-.]+$/;

function LabelSelector({ onChange }: LabelSelectorProps) {
  const [labels, setLabels] = useState<string[]>([]);
  const [showAdd, setShowAdd] = useState(false);
  const [newLabel, setNewLabel] = useState('');
  const [validLabel, setValidLabel] = useState(false);
  const [showTooltip, setShowTooltip] = useState(false);

  const infoIconRef = useRef<HTMLDivElement>();
  const addLabelInputRef = useRef<HTMLInputElement>();
  const addLabelContainerRef = useRef<HTMLDivElement>();

  useEffect(() => {
    setValidLabel(VALID_LABEL.test(newLabel));
  }, [newLabel]);

  useEffect(() => {
    onChange(labels);
  }, [labels]);

  useEffect(() => {
    if (showAdd && addLabelInputRef.current) {
      addLabelInputRef.current.focus();
    }
  }, [showAdd]);

  const clickOutsideHandler = useCallback(() => setShowAdd(false), []);
  const escapeHandler = useCallback(() => setShowAdd(false), []);

  useClickOutside(addLabelContainerRef, clickOutsideHandler);
  useEscape(escapeHandler);

  const handleAddLabel = () => {
    setLabels([...labels, newLabel.trim()]);
    setNewLabel('');
  };

  const handleLabelDismiss = (label: string) => {
    const labelList = [...labels];
    labelList.splice(labelList.indexOf(label), 1);
    setLabels(labelList);
  };

  return (
    <div>
      <Heading>
        <Flex justifyContent="space-between">
          <Flex>
            <Text>Assign Labels (optional)</Text>
            <div ref={infoIconRef} style={{ marginLeft: '12px' }}>
              <Info
                style={{
                  cursor: 'pointer',
                  fontSize: '16px',
                  paddingTop: '5px',
                }}
                onClick={() => setShowTooltip(!showTooltip)}
              />
            </div>
            <Popover
              id="simple-popper"
              open={showTooltip}
              anchorOrigin={{
                vertical: 'bottom',
                horizontal: 'right',
              }}
              transformOrigin={{
                vertical: 'top',
                horizontal: 'left',
              }}
              anchorEl={infoIconRef.current}
              onClose={() => setShowTooltip(false)}
            >
              <Box
                bg="#011223"
                color="white"
                width={362}
                p={4}
                style={{
                  boxShadow: '0px 8px 14px rgba(12, 12, 14, 0.07)',
                  borderRadius: '8px',
                }}
              >
                Teleport provides users the ability to add labels (in the form
                of key:value pairs) to resources. Some valid example labels are
                “env: prod” and “arch: x86_64”. Labels, used in conjunction with
                roles, define access in Teleport. For example, you can specify
                that users with the “on-call” role can access resources labeled
                “env: prod”. For more information, check out our documentation
                on{' '}
                <Link
                  href="https://goteleport.com/docs/setup/admin/trustedclusters/"
                  target="_blank"
                >
                  RBAC
                </Link>{' '}
                and{' '}
                <Link
                  href="https://goteleport.com/docs/setup/admin/labels/"
                  target="_blank"
                  rel="noreferrer"
                >
                  labels
                </Link>
                .
              </Box>
            </Popover>
          </Flex>
          <Text>
            <Link
              href="https://goteleport.com/docs/setup/admin/labels/"
              target="_blank"
              color="rgb(255, 255, 255)"
            >
              View Documentation
            </Link>
          </Text>
        </Flex>
      </Heading>
      <LabelContainer
        onClick={() => setShowAdd(!showAdd)}
        data-testid="label-container"
      >
        {labels.length === 0 && (
          <Text color="rgba(255, 255, 255, 0.1)">Click to add new labels.</Text>
        )}
        {labelList({ labels, onDismiss: handleLabelDismiss })}
      </LabelContainer>
      {showAdd && (
        <AddLabelContainer
          ref={addLabelContainerRef}
          data-testid="add-label-container"
        >
          <AddLabelInput
            name="addLabel"
            value={newLabel}
            onChange={e => {
              setNewLabel(e.target.value);
            }}
            onKeyPress={e => {
              // Add a new label on `Enter` if it's valid.
              if (e.key === 'Enter' && validLabel) {
                handleAddLabel();
              }
            }}
            ref={addLabelInputRef}
          />
          {validLabel ? (
            <CreateLabel
              onClick={handleAddLabel}
              data-testid="create-label-msg"
            >
              + Create new label "{newLabel}"
            </CreateLabel>
          ) : (
            <CreateLabelError data-testid="create-label-error">
              <WarningIconWrapper>
                <Warning style={{ padding: '3px' }} />
              </WarningIconWrapper>
              <WarningText>
                <Text style={{ color: '#D83C31', fontWeight: 700 }}>
                  Invalid label format
                </Text>
                <Text>Follow `key:pair` format to add a new label</Text>
              </WarningText>
            </CreateLabelError>
          )}
        </AddLabelContainer>
      )}
    </div>
  );
}

const Heading = styled.div`
  height: 1.5rem;
  position: relative;
`;

const LabelContainer = styled.div`
  border-radius: 4px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  box-shadow: 0px 8px 10px rgba(12, 12, 14, 0.07);
  cursor: pointer;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 8px;
  min-height: 36px;
  padding: 10px 16px;
`;

const AddLabelContainer = styled.div`
  background: #182250;
  border-radius: 4px;
  height: 100px;
  padding: 1rem;
`;

const AddLabelInput = styled.input`
  background: #182250;
  border-radius: 52px;
  border: 1.5px solid ${({ theme }) => theme.colors.brand.main};
  color: white;
  height: 40px;
  padding: 0 12px;
  width: calc(100% - 2rem);
`;

const CreateLabel = styled.button`
  background: none;
  border: none;
  color: white;
  cursor: pointer;
  font-size: 1rem;
  margin-left: 16px;
  margin-top: 25px;
`;

const CreateLabelError = styled.div`
  display: flex;
  margin-top: 8px;
`;

const WarningIconWrapper = styled.div`
  background: rgba(255, 255, 255, 0.05);
  border-radius: 54px;
  display: flex;
  height: 20px;
  margin-top: 10px;
  padding: 5px;
  text-align: center;
  width: 20px;
`;

const WarningText = styled.div`
  margin-left: 8px;
`;

type LabelSelectorProps = {
  onChange: (labels: string[]) => void;
};

function labelList({
  labels,
  onDismiss,
}: {
  labels: string[];
  onDismiss: (string) => void;
}) {
  return labels.map(label => (
    <Pill key={label} label={label} onDismiss={onDismiss} />
  ));
}

export { LabelSelector };
