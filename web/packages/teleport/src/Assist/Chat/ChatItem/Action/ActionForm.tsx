import React, { useCallback, useState } from 'react';
import styled from 'styled-components';

import { LabelIcon, ServerIcon, UserIcon } from 'design/SVGIcon';
import { Cross } from 'design/Icon';

import { ExecuteRemoteCommandContent, Label, Type } from 'teleport/Assist/services/messages';
import { ActionState } from 'teleport/Assist/Chat/ChatItem/Action/types';

import { Container, getTextForType, Items, Title } from './common';

interface ActionFormProps {
  initialState: ActionState[];
  addNodes?: boolean;
  onSave: (state: ActionState[]) => void;
  onCancel: () => void;
}

const CommandInput = styled.input`
  padding: 10px 15px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 5px;
  font-size: 16px;
  font-family: SFMono-Regular, Consolas, Liberation Mono, Menlo, Courier,
    monospace;
  font-weight: bold;
  border: none;
  color: white;
  width: 100%;
  box-sizing: border-box;

  &:focus {
    outline: none;
  }
`;

const CancelButton = styled.div`
  font-weight: bold;
  border-radius: 5px;
  padding: 5px 15px;
  display: inline-flex;
  align-self: flex-end;
  cursor: pointer;
  margin-right: 10px;
  
  &:hover {
    background: rgba(255, 255, 255, 0.1);
  }
`;

const SaveButton = styled.div`
  margin-top: 10px;
  background: #5130c9;
  font-weight: bold;
  border-radius: 5px;
  padding: 5px 15px;
  display: inline-flex;
  align-self: flex-end;
  cursor: pointer;
`;

const LabelForm = styled.div`
  display: flex;
  background: rgba(255, 255, 255, 0.1);
  align-items: center;
  padding: 1px 15px;
  border-radius: 5px;
  margin-right: 10px;
`;

const LabelFormContent = styled.div`
  display: flex;
  align-items: center;
`;

const Input = styled.input`
  background: transparent;
  padding: 10px 15px;
  border-radius: 5px;
  margin-right: 10px;
  font-size: 16px;
  font-weight: bold;
  border: none;
  width: 140px;
  box-sizing: border-box;

  &:focus {
    outline: none;
  }
`;

const DeleteButton = styled.div`
  padding: 1px 4px;
  border-radius: 5px;
  cursor: pointer;
  justify-self: flex-end;

  &:hover {
    background: rgba(255, 255, 255, 0.2);
  }
`;


const FormFooter = styled.div`
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
`;

const FooterAdd = styled.div`
  display: flex;
  margin-left: -8px;
`;

const FooterAddButton = styled.div`
  margin-right: 10px;
  padding: 5px 8px;
  border-radius: 5px;
  cursor: pointer;
  color: rgba(255, 255, 255, 0.7);

  &:hover {
    background: rgba(255, 255, 255, 0.1);
  }
`;

const FooterButtons = styled.div``;

const As = styled.div`
  padding: 10px 15px;
`;

export function ActionForm(props: ActionFormProps) {
  const [formState, setFormState] = useState<ActionState[]>(props.initialState);

  const handleChange = useCallback((index: number, value: any) => {
    setFormState(existing =>
      existing.map((item, i) => {
        if (index === i) {
          return {
            ...item,
            value,
          };
        }

        return item;
      })
    );
  }, []);

  const handleLabelChange = useCallback(
    (index: number, key: string, value: any) => {
      setFormState(existing =>
        existing.map((item, i) => {
          if (index === i) {
            return {
              type: 'label',
              value: {
                ...(item.value as Label),
                [key]: value,
              },
            };
          }

          return item;
        })
      );
    },
    []
  );

  const handleDelete = useCallback(index => {
    setFormState(existing => existing.filter((item, i) => i !== index));
  }, []);

  const handleAddNode = useCallback(() => {
    setFormState(existing => [
      ...existing,
      {
        type: 'node',
        value: 'node',
      },
    ]);
  }, []);

  const handleAddLabel = useCallback(() => {
    setFormState(existing => [
      ...existing,
      {
        type: 'label',
        value: { key: 'key', value: 'value' },
      },
    ]);
  }, []);

  const items = [];

  for (const [index, stateItem] of formState.entries()) {
    console.log('stateItem', stateItem);
    if (stateItem.type === 'command') {
      items.push(
        <CommandInput
          autoFocus
          key="command"
          value={stateItem.value}
          onChange={event => handleChange(index, event.target.value)}
        />
      );
    }

    if (stateItem.type === 'label') {
      items.push(
        <LabelForm key={`label-${index}`}>
          <LabelFormContent>
            <LabelIcon />

            <Input
              key="2"
              value={stateItem.value.key}
              onChange={event =>
                handleLabelChange(index, 'key', event.target.value)
              }
              style={{ color: 'rgba(255, 255, 255, 0.6)' }}
            />

            <Input
              key="1"
              value={stateItem.value.value}
              onChange={event =>
                handleLabelChange(index, 'value', event.target.value)
              }
              style={{ color: 'white' }}
            />
          </LabelFormContent>

          <DeleteButton onClick={() => handleDelete(index)}>
            <Cross />
          </DeleteButton>
        </LabelForm>
      );
    }

    if (stateItem.type === 'node') {
      items.push(
        <LabelForm key={`node-${index}`}>
          <LabelFormContent>
            <ServerIcon size={16} />

            <Input
              key="node"
              value={stateItem.value}
              onChange={event => handleChange(index, event.target.value)}
              style={{ color: 'white' }}
            />
          </LabelFormContent>

          <DeleteButton onClick={() => handleDelete(index)}>
            <Cross />
          </DeleteButton>
        </LabelForm>
      );
    }

    if (stateItem.type === 'user') {
      items.push(
        <As key={`as-${index}`}>as</As>,
        <LabelForm key={`user-${index}`}>
          <LabelFormContent>
            <UserIcon size={16} />

            <Input
              key="command"
              value={stateItem.value}
              onChange={event => handleChange(index, event.target.value)}
              style={{ color: 'white' }}
            />
          </LabelFormContent>

          <DeleteButton onClick={() => handleDelete(index)}>
            <Cross />
          </DeleteButton>
        </LabelForm>
      );
    }
  }

  return (
    <Container>
      <Items>{items}</Items>

      <FormFooter>
        <FooterAdd>
          {props.addNodes && (
            <>
              <FooterAddButton onClick={() => handleAddLabel()}>
                Add label
              </FooterAddButton>

              <FooterAddButton onClick={() => handleAddNode()}>
                Add node
              </FooterAddButton>
            </>
          )}
        </FooterAdd>

        <FooterButtons>
          <CancelButton onClick={() => props.onCancel()}>Cancel</CancelButton>
          <SaveButton onClick={() => props.onSave(formState)}>Save</SaveButton>
        </FooterButtons>
      </FormFooter>
    </Container>
  );
}
