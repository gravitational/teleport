import { tsh } from 'teleterm/ui/services/clusters/types';

type SuggestionBase<T, R> = {
  kind: T;
  token: string;
  appendToToken?: string;
  data: R;
};

export type SuggestionCmd = SuggestionBase<
  'suggestion.cmd',
  { name: string; displayName: string; description: string }
>;

export type SuggestionSshLogin = SuggestionBase<
  'suggestion.ssh-login',
  string
> & { appendToToken: string };

export type SuggestionServer = SuggestionBase<'suggestion.server', tsh.Server>;

export type Suggestion = SuggestionCmd | SuggestionSshLogin | SuggestionServer;

export type QuickInputPicker = {
  onPick(suggestion: Suggestion): void;
  getAutocompleteResult(input: string, startIndex: number): AutocompleteResult;
};

export type AutocompleteToken = {
  value: string;
  startIndex: number;
};

type AutocompleteResultBase<T> = {
  kind: T;
  // Command includes the result of parsing whatever was parsed so far.
  // This means that in case of `tsh ssh roo`, the command will say that we want to launch `tsh ssh`
  // with `roo` as `loginHost`.
  command: AutocompleteCommand;
};

export type AutocompletePartialMatch =
  AutocompleteResultBase<'autocomplete.partial-match'> & {
    suggestions: Suggestion[];
    targetToken: AutocompleteToken;
  };

export type AutocompleteNoMatch =
  AutocompleteResultBase<'autocomplete.no-match'>;

export type AutocompleteResult = AutocompletePartialMatch | AutocompleteNoMatch;

type CommandBase<T> = {
  kind: T;
};

export type AutocompleteUnknownCommand = CommandBase<'command.unknown'>;

export type AutocompleteTshSshCommand = CommandBase<'command.tsh-ssh'> & {
  loginHost: string;
};

export type AutocompleteCommand =
  | AutocompleteUnknownCommand
  | AutocompleteTshSshCommand;
