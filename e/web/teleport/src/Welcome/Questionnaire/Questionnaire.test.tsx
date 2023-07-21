import React from 'react';
import { fireEvent, render, screen, userEvent } from 'design/utils/testing';

import { userEventService } from 'teleport/services/userEvent';

import { Questionnaire } from './Questionnaire';
import { QuestionnaireProps } from './types';

describe('questionnaire', () => {
  let props: QuestionnaireProps;

  beforeEach(() => {
    props = {
      full: false,
      username: '',
    };

    jest.spyOn(userEventService, 'capturePreUserEvent');
  });

  test('loads each question', () => {
    props.full = true;
    render(<Questionnaire {...props} />);

    expect(screen.getByText('Tell us about yourself')).toBeVisible();
    expect(screen.getByLabelText('Company Name')).toBeInTheDocument();
    expect(screen.getByLabelText('Number of Employees')).toBeInTheDocument();
    expect(screen.getByLabelText('Which Team are you on?')).toBeInTheDocument();
    expect(screen.getByLabelText('Job Title')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Which infrastructure resources do you need to access frequently?'
      )
    ).toBeInTheDocument();
  });

  test('skips questions if not full', () => {
    props.full = false;
    render(<Questionnaire {...props} />);

    expect(screen.getByText('Tell us about yourself')).toBeInTheDocument();

    expect(screen.queryByLabelText('Company Name')).not.toBeInTheDocument();
    expect(
      screen.queryByLabelText('Number of Employees')
    ).not.toBeInTheDocument();
    expect(screen.getByLabelText('Which Team are you on?')).toBeInTheDocument();
    expect(screen.getByLabelText('Job Title')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Which infrastructure resources do you need to access frequently?'
      )
    ).toBeInTheDocument();
  });

  test('shows validation errors', async () => {
    props.full = true;
    render(<Questionnaire {...props} />);

    expect(screen.getByText('Tell us about yourself')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: /Submit/i }));

    expect(
      screen.getByLabelText('Company Name is required')
    ).toBeInTheDocument();
    expect(
      screen.getByLabelText('Number of Employees is required')
    ).toBeInTheDocument();
    expect(screen.getByLabelText('Team is required')).toBeInTheDocument();
    expect(screen.getByLabelText('Job Title is required')).toBeInTheDocument();
    expect(screen.getByText('Resource is required')).toBeInTheDocument();

    // assert data was not saved to local storage
    expect(
      JSON.parse(localStorage.getItem('grv_teleport_onboard_survey'))
    ).toBeNull();

    // assert prehog event was not triggered
    expect(userEventService.capturePreUserEvent).not.toHaveBeenCalled();
  });

  test('submits responses', async () => {
    props.full = true;
    props.username = 'user-000';
    render(<Questionnaire {...props} />);

    expect(screen.getByText('Tell us about yourself')).toBeInTheDocument();

    const companyNameInput: HTMLInputElement =
      screen.getByLabelText('Company Name');
    fireEvent.change(companyNameInput, { target: { value: 'Teleport' } });
    expect(companyNameInput.value).toBe('Teleport');

    await userEvent.click(screen.getByText(/Select Company Size/i));
    await userEvent.click(screen.getByText(/5000+/i));

    await userEvent.click(screen.getByText(/Select Team/i));
    await userEvent.click(screen.getByText(/Legal/i));

    await userEvent.click(screen.getByText(/Select Job Title/i));
    await userEvent.click(screen.getByText(/VP/i));

    await userEvent.click(screen.getByText(/Applications/i));
    await userEvent.click(screen.getByText(/Desktops/i));
    await userEvent.click(screen.getByText(/Kubernetes/i));

    await userEvent.click(screen.getByRole('button', { name: /Submit/i }));

    // assert data is saved to local storage
    expect(
      JSON.parse(localStorage.getItem('grv_teleport_onboard_survey'))
    ).toEqual(
      expect.objectContaining({
        companyName: 'Teleport',
        employeeCount: '5000+',
        clusterResources: [5, 1, 4],
        resources: [
          'RESOURCE_WEB_APPLICATIONS',
          'RESOURCE_WINDOWS_DESKTOPS',
          'RESOURCE_KUBERNETES',
        ],
        role: 'VP',
        team: 'LEGAL',
      })
    );
    localStorage.clear();

    // assert posthog event triggered
    expect(userEventService.capturePreUserEvent).toHaveBeenCalledWith({
      event: 'tp.ui.onboard.questionnaire.submit',
      username: 'user-000',
    });
  });
});
