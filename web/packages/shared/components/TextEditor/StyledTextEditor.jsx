/*
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

import styled from 'styled-components';

import { color } from 'design/system';

const StyledTextEditor = styled.div`
  overflow: hidden;
  border-radius: 4px;
  flex: 1;
  display: flex;
  position: relative;
  border: none;

  ${color};

  > .ace_editor {
    position: absolute;
    top: 8px;
    right: 0;
    bottom: 0;
    left: 0;
  }

  // Theme based on Tomorrow Night Blue theme
  .ace-teleport {
    color: ${props => props.theme.colors.text.main};
  }

  /* Editor gutter contains of line numbers and annotation marks. */

  .ace-teleport .ace_gutter {
    color: ${props => props.theme.colors.text.muted};
  }

  .ace-teleport .ace_constant.ace_other,
  .ace-teleport .ace_cursor {
    color: ${props => props.theme.colors.text.main};
  }

  .ace-teleport .ace_marker-layer .ace_selection {
    background-color: ${props => props.theme.colors.spotBackground[2]};
  }

  .ace-teleport.ace_multiselect .ace_selection.ace_start {
    box-shadow: 0 0 3px 0 ${props => props.theme.colors.levels.sunken};
  }

  /* Debugger line,  NOT RELEVANT FOR YAML  */

  .ace-teleport .ace_marker-layer .ace_step {
    background-color: ${props => props.theme.colors.terminal.brightYellow};
  }

  .ace-teleport .ace_marker-layer .ace_bracket {
    margin: -1px 0 0 -1px;
    border: 1px solid ${props => props.theme.colors.levels.popout};
  }

  /* Background color of active editor's line */

  .ace-teleport .ace_marker-layer .ace_active-line {
    background-color: ${props => props.theme.colors.spotBackground[1]};
  }

  /* Background color of gutter active line */

  .ace-teleport .ace_gutter-active-line {
    background-color: inherit;
  }

  /* Style of selected words. Try it by double clicking on any word in Result tab*/

  .ace-teleport .ace_marker-layer .ace_selected-word {
    box-shadow: 0 0 0 1px ${props => props.theme.colors.text.main};
    border-radius: 2px;
  }

  /* We just want to make it a hair wider without breaking the layout */

  .ace-teleport .ace_fold_widget {
    width: 13px;
    margin-right: -14px;
    background-color: ${props => props.theme.colors.spotBackground[1]};
  }

  .ace-teleport .ace_invisible {
    color: ${props => props.theme.colors.levels.popout};
  }

  .ace-teleport .ace_keyword,
  .ace-teleport .ace_meta,
  .ace-teleport .ace_storage,
  .ace-teleport .ace_storage.ace_type,
  .ace-teleport .ace_support.ace_type {
    color: ${props => props.theme.colors.editor.purple};
  }

  .ace-teleport .ace_keyword.ace_operator {
    color: ${props => props.theme.colors.editor.cyan};
  }

  .ace-teleport .ace_constant.ace_character,
  .ace-teleport .ace_constant.ace_language,
  .ace-teleport .ace_constant.ace_numeric,
  .ace-teleport .ace_keyword.ace_other.ace_unit,
  .ace-teleport .ace_support.ace_constant,
  .ace-teleport .ace_variable.ace_parameter {
    color: ${props => props.theme.colors.editor.abbey};
  }

  .ace-teleport .ace_invalid {
    color: ${props => props.theme.colors.text.main};
    background-color: ${props =>
      props.theme.colors.dataVisualisation.secondary.abbey};
  }

  .ace-teleport .ace_invalid.ace_deprecated {
    color: ${props => props.theme.colors.text.main};
    background-color: ${props => props.theme.colors.editor.purple};
  }

  .ace-teleport .ace_fold {
    border-color: ${props => props.theme.colors.text.main};
    background-color: ${props => props.theme.colors.editor.purple};
  }

  .ace-teleport .ace_entity.ace_name.ace_function,
  .ace-teleport .ace_support.ace_function,
  .ace-teleport .ace_variable {
    color: ${props => props.theme.colors.editor.picton};
  }

  .ace-teleport .ace_support.ace_class,
  .ace-teleport .ace_support.ace_type {
    color: ${props => props.theme.colors.editor.sunflower};
  }

  .ace-teleport .ace_heading,
  .ace-teleport .ace_markup.ace_heading,
  .ace-teleport .ace_string {
    color: ${props => props.theme.colors.editor.caribbean};
  }

  .ace-teleport .ace_entity.ace_name.ace_tag,
  .ace-teleport .ace_entity.ace_other.ace_attribute-name,
  .ace-teleport .ace_meta.ace_tag,
  .ace-teleport .ace_string.ace_regexp,
  .ace-teleport .ace_variable {
    color: ${props => props.theme.colors.editor.purple};
  }

  .ace-teleport .ace_comment {
    color: ${props => props.theme.colors.text.muted};
  }

  /* End: different token styles */

  .ace-teleport .ace_indent-guide {
    /* Indent guide style */
    background: url(data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAACCAYAAACZgbYnAAAAEklEQVQImWNgYGBgYHB3d/8PAAOIAdULw8qMAAAAAElFTkSuQmCC)
      right repeat-y;
  }

  .ace-teleport .ace_indent-guide-active {
    /* Active indent guide style */
    background: url('data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAACAQMAAACjTyRkAAAABlBMVEUAAADCwsK76u2xAAAAAXRSTlMAQObYZgAAAAxJREFUCNdjYGBoAAAAhACBGFbxzQAAAABJRU5ErkJggg==')
      right repeat-y;
  }
`;

export default StyledTextEditor;
