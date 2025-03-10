/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { useEffect, useRef, useState } from 'react';
import { components } from 'react-select';
import ReactSelectCreatable from 'react-select/creatable';
import styled from 'styled-components';

import { Box, ButtonBorder, ButtonIcon, Flex, Text } from 'design';
import * as Icon from 'design/Icon';

import { ReviewerOption } from './types';

export function SelectReviewers({
  reviewers,
  selectedReviewers,
  setSelectedReviewers,
}) {
  const selectWrapperRef = useRef(null);
  const reactSelectRef = useRef(null);
  const [editReviewers, setEditReviewers] = useState(false);
  const [suggestedReviewers, setSuggestedReviewers] = useState<
    ReviewerOption[]
  >(
    // Initially, all suggested reviewers are selected for the requestor.
    () => reviewers.map(r => ({ value: r, label: r, isDisabled: true }))
  );

  useEffect(() => {
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
              <Icon.CircleCheck size="medium" color="success.main" mr={2} />
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

  function handleOnChange(values: ReviewerOption[]) {
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
          value={selectedReviewers}
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
        updateReviewers={handleOnChange}
      />
    </Box>
  );
}

function Reviewers({
  reviewers,
  editReviewers,
  toggleEditReviewers,
  updateReviewers,
}: {
  reviewers: ReviewerOption[];
  editReviewers: boolean;
  toggleEditReviewers(): void;
  updateReviewers(o: ReviewerOption[]): void;
}) {
  const [expanded, setExpanded] = useState(true);
  const ArrowIcon = expanded ? Icon.ChevronDown : Icon.ChevronRight;

  const $reviewers = reviewers.map((reviewer, index) => {
    return (
      <Flex
        border={1}
        borderColor="levels.surface"
        borderRadius={1}
        px={3}
        py={2}
        alignItems="center"
        justifyContent="space-between"
        key={index}
        css={`
          background: ${props => props.theme.colors.spotBackground[0]};
        `}
      >
        <Text
          typography="body3"
          bold
          style={{ whiteSpace: 'nowrap', maxWidth: '200px' }}
          title={reviewer.value}
        >
          {reviewer.value}
        </Text>
        <ButtonIcon
          size={0}
          title="Remove reviewer"
          onClick={() =>
            updateReviewers(reviewers.filter(r => r.value != reviewer.value))
          }
        >
          <Icon.Cross size={16} />
        </ButtonIcon>
      </Flex>
    );
  });

  let btnTxt = 'Add';
  if (reviewers.length > 0) {
    btnTxt = 'Edit';
  }
  if (editReviewers) {
    btnTxt = 'Done';
  }

  return (
    <>
      <Flex
        borderBottom={1}
        mb={2}
        pb={2}
        justifyContent="space-between"
        alignItems="center"
        height="34px"
        css={`
          border-color: ${props => props.theme.colors.spotBackground[1]};
        `}
      >
        <Flex alignItems="baseline" gap={2}>
          <Text typography="body3">Reviewers (optional)</Text>
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
            {btnTxt}
          </ButtonBorder>
          {reviewers.length > 0 ? (
            <ButtonBorder
              onClick={() => updateReviewers([])}
              size="small"
              width="50px"
            >
              Clear
            </ButtonBorder>
          ) : null}
        </Flex>
        {reviewers.length > 0 && (
          <ButtonIcon onClick={() => setExpanded(e => !e)}>
            <ArrowIcon size="medium" />
          </ButtonIcon>
        )}
      </Flex>
      {expanded && <Box data-testid="reviewers">{$reviewers}</Box>}
    </>
  );
}

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
      color: ${props => props.theme.colors.success.main};
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
