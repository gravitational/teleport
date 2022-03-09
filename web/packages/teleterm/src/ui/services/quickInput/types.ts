import { tsh } from 'teleterm/ui/services/clusters/types';

type SuggestionBase<T, R> = {
  kind: T;
  token: string;
  data: R;
};

export type SuggestionCmd = SuggestionBase<
  'suggestion.cmd',
  { name: string; displayName: string; description: string }
>;

export type SuggestionSshLogin = SuggestionBase<'suggestion.ssh-login', null>;

export type QuickInputPicker = {
  onPick(suggestion: Suggestion): void;
  getAutocompleteResult(input: string): AutocompleteResult;
};

export type Suggestion = SuggestionCmd | SuggestionSshLogin;

type AutocompleteBase<T> = {
  kind: T;
  picker: QuickInputPicker;
};

export type AutocompletePartialMatch =
  AutocompleteBase<'autocomplete.partial-match'> & {
    suggestions: Suggestion[];
  };

export type AutocompleteNoMatch = AutocompleteBase<'autocomplete.no-match'>;

export type AutocompleteResult = AutocompletePartialMatch | AutocompleteNoMatch;
