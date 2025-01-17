/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useCallback, useEffect, useRef, useState } from 'react';
import styled from 'styled-components';

import { Box, Flex, Link, Pill, Popover, Text } from 'design';
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
                size="medium"
                onClick={() => setShowTooltip(!showTooltip)}
              />
            </div>
            <Popover
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
              <Box bg="levels.elevated" width={362} p={4}>
                Teleport provides users the ability to add labels (in the form
                of key:value pairs) to resources. Some valid example labels are
                “env: prod” and “arch: x86_64”. Labels, used in conjunction with
                roles, define access in Teleport. For example, you can specify
                that users with the “on-call” role can access resources labeled
                “env: prod”. For more information, check out our documentation
                on{' '}
                <Link
                  href="https://goteleport.com/docs/admin-guides/management/admin/trustedclusters/"
                  target="_blank"
                >
                  RBAC
                </Link>{' '}
                and{' '}
                <Link
                  href="https://goteleport.com/docs/admin-guides/management/admin/labels/"
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
              href="https://goteleport.com/docs/admin-guides/management/admin/labels/"
              target="_blank"
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
          <Text color="text.muted">Click to add new labels.</Text>
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
                <Warning style={{ padding: '1px' }} size="medium" />
              </WarningIconWrapper>
              <WarningText>
                <Text color="error.main" style={{ fontWeight: 700 }}>
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
  border: 1px solid ${props => props.theme.colors.spotBackground[1]};
  box-shadow: ${props => props.theme.boxShadow[2]};
  cursor: pointer;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 8px;
  min-height: 36px;
  padding: 10px 16px;
`;

const AddLabelContainer = styled.div`
  border-radius: 4px;
  height: 100px;
  padding: 1rem;
`;

const AddLabelInput = styled.input`
  border-radius: 52px;
  border: 1.5px solid ${({ theme }) => theme.colors.brand};
  color: ${({ theme }) => theme.colors.text.main};
  height: 40px;
  padding: 0 12px;
  width: calc(100% - 2rem);
`;

const CreateLabel = styled.button`
  background: none;
  border: none;
  color: ${({ theme }) => theme.colors.text.main};
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
  background: ${props => props.theme.colors.spotBackground[0]};
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
