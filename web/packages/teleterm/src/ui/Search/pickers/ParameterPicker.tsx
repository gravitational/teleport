/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
            <Highlight text={item} keywords={[inputValue]}></Highlight>
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
