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

import React, { ReactElement, useCallback, useEffect } from 'react';
import { Highlight } from 'shared/components/Highlight';
import {
  makeSuccessAttempt,
  mapAttempt,
  useAsync,
} from 'shared/hooks/useAsync';
import { Text } from 'design';
import * as icons from 'design/Icon';

import { useSearchContext } from '../SearchContext';
import { ParametrizedAction } from '../actions';

import { IconAndContent, NonInteractiveItem, ResultList } from './ResultList';
import { actionPicker } from './pickers';
import { PickerContainer } from './PickerContainer';

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
  const inputSuggestionAttempt = makeSuccessAttempt(inputValue && [inputValue]);
  const $suggestionsError =
    suggestionsAttempt.status === 'error' ? (
      <SuggestionsError statusText={suggestionsAttempt.statusText} />
    ) : null;

  useEffect(() => {
    getSuggestions();
    // We want to get suggestions only once on mount.
    // useAsync already handles cleanup and calling the hook twice.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const attempt = mapAttempt(suggestionsAttempt, suggestions =>
    suggestions.filter(
      v =>
        v.toLocaleLowerCase().includes(inputValue.toLocaleLowerCase()) &&
        v !== inputValue
    )
  );

  const onPick = useCallback(
    (item: string) => {
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
      <ResultList<string>
        attempts={[inputSuggestionAttempt, attempt]}
        ExtraTopComponent={$suggestionsError}
        onPick={onPick}
        onBack={onBack}
        addWindowEventListener={addWindowEventListener}
        render={item => ({
          key: item,
          Component: (
            <Text typography="body1" fontSize={1}>
              <Highlight text={item} keywords={[inputValue]}></Highlight>
            </Text>
          ),
        })}
      />
    </PickerContainer>
  );
}

export const SuggestionsError = ({ statusText }: { statusText: string }) => (
  <NonInteractiveItem>
    <IconAndContent Icon={icons.Warning} iconColor="warning.main">
      <Text typography="body1">
        Could not fetch suggestions. Type in the desired value to continue.
      </Text>
      <Text typography="body2">{statusText}</Text>
    </IconAndContent>
  </NonInteractiveItem>
);
