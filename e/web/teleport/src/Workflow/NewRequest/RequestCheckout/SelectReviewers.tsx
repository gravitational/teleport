import React, { useState, useRef } from 'react';
import { components } from 'react-select';
import ReactSelectCreatable from 'react-select/creatable';
import styled from 'styled-components';
import { ButtonBorder, Box, Text, Flex } from 'design';
import * as Icon from 'design/Icon';
import { Option } from 'shared/components/Select';

export function SelectReviewers({
  reviewers,
  selectedReviewers,
  setSelectedReviewers,
}) {
  const selectWrapperRef = useRef(null);
  const reactSelectRef = useRef(null);
  const [editReviewers, setEditReviewers] = useState(false);
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
            <Flex alignItems="center" width="210px">
              <Icon.CircleCheck size="medium" color="success" mr={2} />
              <Text title={props.data.value}>{props.data.value}</Text>
            </Flex>
            <Icon.Cross size="small" />
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

  function toggleEditReviewers() {
    setEditReviewers(!editReviewers);
  }

  return (
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
        borderColor="levels.surface"
        borderRadius={1}
        px={3}
        py={2}
        mb={2}
        alignItems="center"
        justifyContent="space-between"
        key={index}
        css={`
          background: ${props => props.theme.colors.spotBackground[0]};
        `}
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
        mb={3}
        pb={3}
        width="260px"
        justifyContent="space-between"
        alignItems="center"
        height="34px"
        css={`
          border-color: ${props => props.theme.colors.spotBackground[1]};
        `}
      >
        <Text mr={2} fontSize={1}>
          Reviewers (optional)
        </Text>
        <ButtonBorder
          onClick={e => {
            // By stopping propagation,
            // we prevent this event from being interpreted as an outside click.
            e.stopPropagation();
            toggleEditReviewers();
          }}
          size="small"
          width="50px"
        >
          {editReviewers ? 'Done' : 'Add'}
        </ButtonBorder>
      </Flex>
      {$reviewers}
    </>
  );
}

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

    .icon-circlecheck {
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
    .icon-circlecheck {
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
