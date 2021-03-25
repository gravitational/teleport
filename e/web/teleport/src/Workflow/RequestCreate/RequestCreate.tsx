import React, { useState, useRef } from 'react';
import { components } from 'react-select';
import ReactSelectCreatable from 'react-select/creatable';
import styled from 'styled-components';
import {
  LabelInput,
  ButtonPrimary,
  ButtonSecondary,
  ButtonBorder,
  Alert,
  Box,
  Text,
  Flex,
} from 'design';
import * as Icon from 'design/Icon';
import Validation, { useRule } from 'shared/components/Validation';
import FieldSelect from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import { requiredField } from 'shared/components/Validation/rules';
import useTeleportE from 'e-teleport/useTeleportE';
import useRequestCreate, { State } from './useRequestCreate';

export default function Container() {
  const ctx = useTeleportE();
  const state = useRequestCreate(ctx);
  return <RequestCreate {...state} />;
}

export function RequestCreate(props: State) {
  const {
    attempt,
    reason,
    requireReason,
    setReason,
    roles,
    reviewers,
    createRequest,
    close,
  } = props;
  const selectWrapperRef = useRef(null);
  const reactSelectRef = useRef(null);
  const [editReviewers, setEditReviewers] = useState(false);
  const [selectedRoles, setSelectedRoles] = useState<Option[]>([]);
  const [roleOptions] = useState<Option[]>(() =>
    roles.map(r => ({ value: r, label: r }))
  );
  const [selectedReviewers, setSelectedReviewers] = useState<CreateOption[]>(
    []
  );
  const [suggestedReviewers, setSuggestedReviewers] = useState<CreateOption[]>(
    () => reviewers.map(r => ({ value: r, label: r, isDisabled: false }))
  );

  React.useEffect(() => {
    // When editing reviewers, auto focus on input box.
    if (editReviewers) {
      reactSelectRef.current.focus();
    }

    // When editing reviewers, clicking outside box closes editor.
    function handleOnClick(e) {
      if (!editReviewers || e.target.closest('.react-select__option')) return;

      if (!selectWrapperRef.current?.contains(e.target)) {
        setEditReviewers(false);
      }
    }

    window.addEventListener('click', handleOnClick);

    return () => {
      window.removeEventListener('click', handleOnClick);
    };
  }, [editReviewers]);

  const reviewerOptions = [
    {
      label: '',
      options: selectedReviewers,
    },
    {
      label: 'Suggested Reviewers',
      options: suggestedReviewers,
    },
  ];

  // formatGroupLabel customizes react-select labels.
  const formatGroupLabel = data => {
    if (!data.label) {
      return null;
    }
    return <SelectGroupLabel>{data.label}</SelectGroupLabel>;
  };

  // Option customizes how react-select options appear.
  const Option = props => {
    if (props.data.isDisabled) {
      return null;
    }

    if (props.data.isSelected) {
      return (
        <components.Option {...props} className="react-select__selected">
          <Flex alignItems="center" justifyContent="space-between">
            <Flex alignItems="center" width="230px">
              <Icon.CircleCheck />
              <Text title={props.data.value}>{props.data.value}</Text>
            </Flex>
            <Icon.Cross />
          </Flex>
        </components.Option>
      );
    }

    return (
      <components.Option {...props}>
        <Text mx={4} title={props.data.value}>
          {props.data.label}
        </Text>
      </components.Option>
    );
  };

  function handleOnChange(values) {
    const updateSelectedReviewers = values.map(r => ({
      value: r.value,
      label: r.label,
      // isSelected flag is used to customize style.
      isSelected: true,
    }));

    const updateSuggestedReviewers = suggestedReviewers.map(r => {
      if (values.find(t => t.value === r.value)) {
        // isDisabled flag is used to not render this name in suggested list.
        r.isDisabled = true;
      } else {
        r.isDisabled = false;
      }
      return r;
    });

    setSelectedReviewers(updateSelectedReviewers);
    setSuggestedReviewers(updateSuggestedReviewers);
  }

  function onCreateRequest(validator) {
    if (!validator.validate()) {
      return;
    }

    const roles = selectedRoles.map(r => r.value);
    const reviewers = selectedReviewers.map(r => r.value);
    createRequest(roles, reviewers);
  }

  function toggleEditReviewers() {
    setEditReviewers(!editReviewers);
  }

  return (
    <Validation>
      {({ validator }) => (
        <Flex>
          <Box
            width="600px"
            p={4}
            pt={3}
            mr={8}
            borderRadius={1}
            bg="primary.main"
            border={1}
            borderColor="primary.light"
          >
            <Text typography="h4" bold mb={3}>
              Request Role Access
            </Text>
            <Box mb={5}>
              {attempt.status === 'failed' && (
                <Alert kind="danger" children={attempt.statusText} />
              )}
              <FieldSelect
                width="300px"
                menuPosition="fixed"
                label="Roles Allowed to Request"
                rule={requiredField('At least one role is required')}
                placeholder="Click to select a role"
                isSearchable
                isMulti
                isSimpleValue
                clearable={false}
                value={selectedRoles}
                onChange={values => setSelectedRoles(values as Option[])}
                options={roleOptions}
              />
              <TextBox
                reason={reason}
                setReason={setReason}
                requireReason={requireReason}
              />
            </Box>
            <Box>
              <ButtonPrimary
                mr="3"
                disabled={attempt.status === 'processing'}
                onClick={() => onCreateRequest(validator)}
              >
                Send Request
              </ButtonPrimary>
              <ButtonSecondary
                disabled={attempt.status === 'processing'}
                onClick={close}
              >
                Cancel
              </ButtonSecondary>
            </Box>
          </Box>
          <Box style={{ position: 'relative' }}>
            <SelectWrapper
              ref={selectWrapperRef}
              style={{ display: editReviewers ? '' : 'none' }}
            >
              <ReactSelectCreatable
                className="react-select-container"
                classNamePrefix="react-select"
                isClearable={false}
                isMulti={true}
                isSearchable={true}
                menuIsOpen={true}
                controlShouldRenderValue={false}
                hideSelectedOptions={false}
                placeholder="Type or select a name"
                options={reviewerOptions}
                onChange={handleOnChange}
                formatGroupLabel={formatGroupLabel}
                components={{ Option }}
                noOptionsMessage={() => null}
                ref={reactSelectRef}
              />
            </SelectWrapper>
            <Reviewers
              reviewers={selectedReviewers}
              editReviewers={editReviewers}
              toggleEditReviewers={toggleEditReviewers}
            />
          </Box>
        </Flex>
      )}
    </Validation>
  );
}

const requireText = (value: string, requireReason: boolean) => () => {
  if (requireReason && (!value || value.trim().length === 0)) {
    return {
      valid: false,
      message: 'Reason Required',
    };
  }
  return { valid: true };
};

function TextBox({ reason, setReason, requireReason }: TextBoxProps) {
  const { valid, message } = useRule(requireText(reason, requireReason));
  const hasError = !valid;
  const labelText = hasError ? message : 'Request Reason';

  const optionalText = requireReason ? '' : ' (optional)';
  const placeholder = `Describe your request...${optionalText}`;

  return (
    <Box>
      <LabelInput hasError={hasError}>{labelText}</LabelInput>
      <Box
        as="textarea"
        height="200px"
        width="500px"
        borderRadius={2}
        p={2}
        border={hasError ? '2px solid' : '0'}
        borderColor={hasError ? 'error.dark' : 'none'}
        style={{ outline: 'none' }}
        placeholder={placeholder}
        value={reason}
        onChange={e => setReason(e.target.value)}
      />
    </Box>
  );
}

function Reviewers({
  reviewers,
  editReviewers,
  toggleEditReviewers,
}: {
  reviewers: CreateOption[];
  editReviewers: boolean;
  toggleEditReviewers(): void;
}) {
  const $reviewers = reviewers.map((reviewer, index) => {
    return (
      <Flex
        border={1}
        borderColor="primary.light"
        borderRadius={1}
        px={3}
        py={2}
        mb={2}
        bg="primary.main"
        alignItems="center"
        justifyContent="space-between"
        key={index}
      >
        <Text
          typography="body2"
          bold
          style={{ whiteSpace: 'nowrap', maxWidth: '200px' }}
          title={reviewer.value}
        >
          {reviewer.value}
        </Text>
      </Flex>
    );
  });

  return (
    <>
      <Flex
        borderBottom={1}
        borderColor="primary.main"
        mb={3}
        pb={3}
        width="260px"
        justifyContent="space-between"
        alignItems="center"
        height="34px"
      >
        <Text typography="h6" mr={2}>
          Reviewers
        </Text>
        <ButtonBorder onClick={toggleEditReviewers} size="small" width="50px">
          {editReviewers ? 'Done' : 'Edit'}
        </ButtonBorder>
      </Flex>
      {$reviewers}
    </>
  );
}

type TextBoxProps = {
  reason: State['reason'];
  setReason: State['setReason'];
  requireReason: State['requireReason'];
};

type CreateOption = Option & {
  isDisabled?: boolean;
  isSelected?: boolean;
};

const SelectWrapper = styled(Box)`
  width: 260px;
  height: 150px;
  background-color: #ffffff;
  color: #000000;
  border-radius: 3px;
  position: absolute;
  z-index: 1;
  top: 40px;

  .react-select__group,
  .react-select__group-heading,
  .react-select__menu-list {
    padding: 0;
    margin: 0;
  }

  .react-select__menu-list {
    margin-top: 10px;
  }

  // Removes auto focus on first option
  .react-select__option--is-focused {
    background-color: inherit;
    &:hover {
      background-color: #deebff;
    }
  }

  .react-select-container {
    width: 300px;
    box-sizing: border-box;
    border: none;
    display: block;
    font-size: 16px;
    outline: none;
    width: 100%;
    background-color: #ffffff;
    margin-top: 16px;
    border-radius: 4px;
  }

  .react-select__menu {
    box-shadow: none;
  }

  .react-select__control {
    border-radius: 30px;
    background-color: #f0f2f4;
    margin: 0px 16px 10px 16px;

    &:hover {
      cursor: pointer;
    }
  }

  .react-select__control--is-focused {
    border-color: transparent;
    box-shadow: none;
  }

  .react-select__placeholder {
    font-size: 14px;
  }

  .react-select__option {
    white-space: nowrap;
    padding: 9px 16px;
    border-top: 1px solid #eaeaea;
    font-weight: bold;
    font-size: 14px;

    &:hover {
      cursor: pointer;

      &:last-child {
        border-bottom-right-radius: 3px;
        border-bottom-left-radius: 3px;
      }
    }

    .icon-checkmark-circle {
      color: transparent;
      margin-right: 10px;
    }
  }

  .react-select__option--is-selected {
    background-color: inherit;
    color: inherit;
  }

  .react-select__indicators {
    display: none;
  }

  .react-select__selected {
    .icon-checkmark-circle {
      color: ${props => props.theme.colors.success};
    }

    .icon-cross {
      color: ${props => props.theme.colors.bgTerminal};
      display: none;
    }

    &:hover .icon-cross {
      display: block;
    }
  }
`;

const SelectGroupLabel = styled(Box)`
  width: 100%;
  background-color: #efefef;
  color: #324148;
  text-transform: none;
  padding: 3px 15px;
`;
