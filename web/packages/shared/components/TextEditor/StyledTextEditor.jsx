/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import styled from 'styled-components';

const StyledTextEditor = styled.div`
  overflow: hidden;
  border-radius: 4px;
  flex: 1;
  display: flex;
  position: relative;
  border: none;
  background-color: ${props => props.theme.colors.levels.sunken};

  > .ace_editor {
    position: absolute;
    top: 8px;
    right: 0;
    bottom: 0;
    left: 0;
  }

  // Theme based on Tomorrow Night Blue theme
  .ace-monokai {
    background-color: ${props => props.theme.colors.levels.sunken};
    color: ${props => props.theme.colors.text.main};
  }

  /* Editor gutter contains of line numbers and annotation marks. */

  .ace-monokai .ace_gutter {
    background-color: ${props => props.theme.colors.levels.sunken};
    color: ${props => props.theme.colors.text.muted};
  }

  .ace-monokai .ace_constant.ace_other,
  .ace-monokai .ace_cursor {
    color: ${props => props.theme.colors.text.muted};
  }

  .ace-monokai .ace_marker-layer .ace_selection {
    background-color: ${props => props.theme.colors.spotBackground[2]};
  }

  .ace-monokai.ace_multiselect .ace_selection.ace_start {
    box-shadow: 0 0 3px 0 ${props => props.theme.colors.levels.sunken};
  }

  /* Debugger line,  NOT RELEVANT FOR YAML  */

  .ace-monokai .ace_marker-layer .ace_step {
    background-color: ${props => props.theme.colors.terminal.brightYellow};
  }

  .ace-tomorrow-night-blue .ace_marker-layer .ace_bracket {
    margin: -1px 0 0 -1px;
    border: 1px solid ${props => props.theme.colors.levels.popout};
  }

  /* Background color of active editor's line */

  .ace-monokai .ace_marker-layer .ace_active-line {
    background-color: ${props => props.theme.colors.spotBackground[1]};
  }

  /* Background color of gutter active line */

  .ace-monokai .ace_gutter-active-line {
    background-color: inherit;
  }

  /* Style of selected words. Try it by double clicking on any word in Result tab*/

  .ace-monokai .ace_marker-layer .ace_selected-word {
    box-shadow: 0 0 0 1px ${props => props.theme.colors.text.main};
    border-radius: 2px;
    border: none;
  }

  /* We just want to make it a hair wider without breaking the layout */

  .ace-monokai .ace_fold_widget {
    width: 13px;
    margin-right: -14px;
    background-color: ${props => props.theme.colors.spotBackground[1]};
  }

  .ace-monokai .ace_invisible {
    color: ${props => props.theme.colors.levels.popout};
  }

  .ace-monokai .ace_keyword,
  .ace-monokai .ace_meta,
  .ace-monokai .ace_storage,
  .ace-monokai .ace_storage.ace_type,
  .ace-monokai .ace_support.ace_type {
    color: ${props => props.theme.colors.editor.purple};
  }

  .ace-monokai .ace_keyword.ace_operator {
    color: ${props => props.theme.colors.terminal.brightCyan};
  }

  .ace-monokai .ace_constant.ace_character,
  .ace-monokai .ace_constant.ace_language,
  .ace-monokai .ace_constant.ace_numeric,
  .ace-monokai .ace_keyword.ace_other.ace_unit,
  .ace-monokai .ace_support.ace_constant,
  .ace-monokai .ace_variable.ace_parameter {
    color: ${props => props.theme.colors.editor.abbey};
  }

  .ace-monokai .ace_invalid {
    color: ${props => props.theme.colors.text.main};
    background-color: ${props =>
      props.theme.colors.dataVisualisation.secondary.abbey};
  }

  .ace-monokai .ace_invalid.ace_deprecated {
    color: ${props => props.theme.colors.text.main};
    background-color: ${props => props.theme.colors.editor.purple};
  }

  .ace-monokai .ace_fold {
    border-color: ${props => props.theme.colors.text.main};
    background-color: ${props => props.theme.colors.editor.purple};
  }

  .ace-monokai .ace_entity.ace_name.ace_function,
  .ace-monokai .ace_support.ace_function,
  .ace-monokai .ace_variable {
    color: ${props => props.theme.colors.editor.picton};
  }

  .ace-monokai .ace_support.ace_class,
  .ace-monokai .ace_support.ace_type {
    color: ${props => props.theme.colors.editor.sunflower};
  }

  .ace-monokai .ace_heading,
  .ace-monokai .ace_markup.ace_heading,
  .ace-monokai .ace_string {
    color: ${props => props.theme.colors.editor.caribbean};
  }

  .ace-monokai .ace_entity.ace_name.ace_tag,
  .ace-monokai .ace_entity.ace_other.ace_attribute-name,
  .ace-monokai .ace_meta.ace_tag,
  .ace-monokai .ace_string.ace_regexp,
  .ace-monokai .ace_variable {
    color: ${props => props.theme.colors.editor.purple};
  }

  .ace-monokai .ace_comment {
    color: ${props => props.theme.colors.text.muted};
  }

  /* End: different token styles */

  .ace-monokai .ace_fold-widget {
    background-image: url('data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAUAAAAFCAYAAACNbyblAAAANElEQVR42mWKsQ0AMAzC8ixLlrzQjzmBiEjp0A6WwBCSPgKAXoLkqSot7nN3yMwR7pZ32NzpKkVoDBUxKAAAAABJRU5ErkJggg==');
    opacity: 0.9;
  }

  .ace-monokai .ace_indent-guide {
    /* Indent guide style */
    background: url(data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAACCAYAAACZgbYnAAAAEklEQVQImWNgYGBgYHB3d/8PAAOIAdULw8qMAAAAAElFTkSuQmCC)
      right repeat-y;
  }

  .ace-monokai .ace_indent-guide-active {
    /* Active indent guide style */
    background: url('data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAACAQMAAACjTyRkAAAABlBMVEUAAADCwsK76u2xAAAAAXRSTlMAQObYZgAAAAxJREFUCNdjYGBoAAAAhACBGFbxzQAAAABJRU5ErkJggg==')
      right repeat-y;
  }
`;

export default StyledTextEditor;
