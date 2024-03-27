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

import React from 'react';
import { Validator } from 'shared/components/Validation';

import { QuestionnaireComponent } from 'teleport/Welcome/Questionnaire';
import { useQuestionnaire } from 'teleport/Welcome/Questionnaire/useQuestionnaire';

/**
 * Endpoint expects form data fields in the following format:
 * curl -X POST https://usage.teleport.dev/productsurvey \
 * -F "title=Eng" \
 * -F "teamsize=200" \
 * -F "team=Development Team" \
 * -F "company=ACME Corp" \
 * -F "access_needs=["RESOURCE_WEB_APPLICATIONS","RESOURCE_KUBERNETES"]
 *
 * Note: New API added to CLI Survey Endpoint.
 * Added to same endpoint as the CURL post install message.
 * https://github.com/gravitational/peopleware/blob/main/rfd/0001-adoption-metrics.md
 */
const PRODUCT_SURVEY_ENDPOINT = 'https://usage.teleport.dev/productsurvey';

export const Questionnaire = ({ onSubmit }: { onSubmit(): void }) => {
  const { formFields, updateForm } = useQuestionnaire();

  const submitForm = async (validator: Validator, isSkipping = false) => {
    if (isSkipping) {
      onSubmit();
      return;
    }

    if (!validator.validate()) {
      return;
    }

    let formData = new FormData();
    formData.set('title', formFields.role);
    formData.set('teamsize', formFields.employeeCount);
    formData.set('team', formFields.team);
    formData.set('company', formFields.companyName);
    formData.set('access_needs', JSON.stringify(formFields.resources));

    // Ignore any errors.
    fetch(PRODUCT_SURVEY_ENDPOINT, {
      method: 'POST',
      body: formData,
    });

    // Callback to continue flow
    onSubmit();
  };

  return (
    <QuestionnaireComponent
      submitForm={submitForm}
      updateForm={updateForm}
      formFields={formFields}
      canSkip={true}
    />
  );
};
