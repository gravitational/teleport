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

import { ReactElement, useCallback, useEffect } from 'react';

import { Text } from 'design';
import * as icons from 'design/Icon';
import { Highlight } from 'shared/components/Highlight';
import {
  Attempt,
  makeSuccessAttempt,
  mapAttempt,
  useAsync,
} from 'shared/hooks/useAsync';

import { Parameter, ParametrizedAction } from '../actions';
import { useSearchContext } from '../SearchContext';
import { PickerContainer } from './PickerContainer';
import { actionPicker } from './pickers';
import { IconAndContent, NonInteractiveItem, ResultList } from './ResultList';

interface ParameterPickerProps {
  action: ParametrizedAction;
  input: ReactElement;
}

export function ParameterPicker(props: ParameterPickerProps) {
  const {
    inputValue,
    close,
    changeActivePicker,
    resetInput,
    addWindowEventListener,
  } = useSearchContext();
  const [suggestionsAttempt, getSuggestions] = useAsync(
    props.action.parameter.getSuggestions
  );
  const inputSuggestionAttempt = makeSuccessAttempt(
    inputValue &&
      !props.action.parameter.allowOnlySuggestions && [
        { value: inputValue, displayText: inputValue },
      ]
  );

  useEffect(() => {
    getSuggestions();
    // We want to get suggestions only once on mount.
    // useAsync already handles cleanup and calling the hook twice.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const matchingSuggestionsAttempt = mapAttempt(
    suggestionsAttempt,
    suggestions =>
      suggestions.filter(
        v =>
          v.displayText
            .toLocaleLowerCase()
            .includes(inputValue.toLocaleLowerCase()) &&
          v.displayText !== inputValue
      )
  );

  const $suggestionsMessage = getSuggestionsMessage({
    matchingSuggestionsAttempt,
    parameter: props.action.parameter,
  });

  const onPick = useCallback(
    (item: Parameter) => {
      props.action.perform(item);

      resetInput();
      if (!props.action.preventAutoClose) {
        close();
      }
    },
    [close, resetInput, props.action]
  );

  const onBack = useCallback(() => {
    changeActivePicker(actionPicker);
  }, [changeActivePicker]);

  return (
    <PickerContainer>
      {props.input}
      <ResultList<Parameter>
        attempts={[inputSuggestionAttempt, matchingSuggestionsAttempt]}
        ExtraTopComponent={$suggestionsMessage}
        onPick={onPick}
        onBack={onBack}
        addWindowEventListener={addWindowEventListener}
        render={item => ({
          key: item.value,
          Component: (
            <Text typography="body2" fontSize={1}>
              <Highlight text={item.displayText} keywords={[inputValue]} />
            </Text>
          ),
        })}
      />
    </PickerContainer>
  );
}

export const SuggestionsError = ({
  statusText,
  allowOnlySuggestions,
}: {
  statusText: string;
  allowOnlySuggestions?: boolean;
}) => (
  <NonInteractiveItem>
    <IconAndContent Icon={icons.Warning} iconColor="warning.main">
      <Text typography="body2">
        Could not fetch suggestions.{' '}
        {!allowOnlySuggestions && 'Type in the desired value to continue.'}
      </Text>
      <Text typography="body3">{statusText}</Text>
    </IconAndContent>
  </NonInteractiveItem>
);

export const NoSuggestionsAvailable = ({ message }: { message: string }) => (
  <NonInteractiveItem>
    <IconAndContent Icon={icons.Info} iconColor="text.slightlyMuted">
      <Text typography="body2">{message}</Text>
    </IconAndContent>
  </NonInteractiveItem>
);

function getSuggestionsMessage({
  matchingSuggestionsAttempt,
  parameter,
}: {
  matchingSuggestionsAttempt: Attempt<Parameter[]>;
  parameter?: ParametrizedAction['parameter'];
}) {
  if (matchingSuggestionsAttempt.status === 'error') {
    return (
      <SuggestionsError statusText={matchingSuggestionsAttempt.statusText} />
    );
  }
  if (
    parameter.allowOnlySuggestions &&
    matchingSuggestionsAttempt.status === 'success' &&
    matchingSuggestionsAttempt.data.length === 0
  ) {
    return (
      <NoSuggestionsAvailable
        message={parameter.noSuggestionsAvailableMessage}
      />
    );
  }
}
