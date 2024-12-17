import {
  fireEvent,
  render,
  screen,
  userEvent,
  waitFor,
} from 'design/utils/testing';

import { userEventService } from 'teleport/services/userEvent';
import api from 'teleport/services/api';
import { mockUserContextProviderWith } from 'teleport/User/testHelpers/mockUserContextWith';
import { makeTestUserContext } from 'teleport/User/testHelpers/makeTestUserContext';

import cfg from 'teleport/config';

import { surveyService } from 'e-teleport/services/survey';

import { EmployeeSelectOptions } from 'e-teleport/Welcome/Questionnaire/constants';

import { Questionnaire } from './Questionnaire';
import { QuestionnaireProps } from './types';

jest.mock('shared/hooks', () => ({
  useAttempt: () => {
    return {
      attempt: { status: 'success', statusText: 'Success Text' },
      setAttempt: jest.fn(),
      run: (fn?: any) => Promise.resolve(fn()),
    };
  },
}));

describe('questionnaire', () => {
  let props: QuestionnaireProps;

  beforeEach(() => {
    props = {
      username: '',
      onboard: true,
    };

    // general mocks:
    jest.spyOn(userEventService, 'capturePreUserEvent');
    jest.spyOn(userEventService, 'captureUserEvent');
    jest
      .spyOn(surveyService, 'getSurveyCompanyResults')
      .mockImplementation(() =>
        Promise.resolve({ companyName: '', employeeCount: '' })
      );

    // non-onboard mocks:
    jest.spyOn(api, 'put').mockImplementation(() => Promise.resolve());
    jest.spyOn(surveyService, 'submitSurvey');
    mockUserContextProviderWith(makeTestUserContext());
  });

  afterEach(() => jest.resetAllMocks());

  test('loads each question', async () => {
    render(<Questionnaire {...props} />);

    await screen.findByText('Tell us about yourself');

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

  test('hides company questions if already answered', async () => {
    jest
      .spyOn(surveyService, 'getSurveyCompanyResults')
      .mockImplementation(() =>
        Promise.resolve({
          companyName: 'Answered Company',
          employeeCount: EmployeeSelectOptions[0].value,
        })
      );

    render(<Questionnaire {...props} />);

    await screen.findByText('Tell us about yourself');

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

  test('preselects resources based on marketing params', async () => {
    jest
      .spyOn(surveyService, 'getSurveyCompanyResults')
      .mockImplementation(() =>
        Promise.resolve({
          companyName: '',
          employeeCount: null,
          marketingParams: {
            campaign: 'foodatabase', // match 'Databases'
            medium: 'desktopbar', // match 'Windows Desktops'
            source: 'kubebaz', // match 'Kubernetes'
            intent: 'sshsssdatabase', // match 'Server/SSH' & 'Databases'
          },
        })
      );

    render(<Questionnaire {...props} />);

    await screen.findByText('Tell us about yourself');
    await waitFor(() => {
      expect(screen.getByTestId('check-RESOURCE_DATABASES')).toBeChecked();
    });

    expect(screen.getByTestId('check-RESOURCE_WINDOWS_DESKTOPS')).toBeChecked();
    expect(screen.getByTestId('check-RESOURCE_KUBERNETES')).toBeChecked();
    expect(screen.getByTestId('check-RESOURCE_SERVER_SSH')).toBeChecked();
    expect(
      screen.getByTestId('check-RESOURCE_WEB_APPLICATIONS')
    ).not.toBeChecked();
  });

  test('shows validation errors', async () => {
    render(<Questionnaire {...props} />);

    await screen.findByText('Tell us about yourself');
    await userEvent.click(screen.getByRole('button', { name: /Submit/i }));

    expect(
      screen.getByRole('textbox', { description: 'Company Name is required' })
    ).toBeInTheDocument();
    expect(screen.getByLabelText('Number of Employees')).toBeInvalid();
    expect(
      screen.getByText('Number of Employees is required')
    ).toBeInTheDocument();
    expect(screen.getByLabelText('Which Team are you on?')).toBeInvalid();
    expect(screen.getByText('Team is required')).toBeInTheDocument();
    expect(screen.getByLabelText('Job Title')).toBeInvalid();
    expect(screen.getByText('Job Title is required')).toBeInTheDocument();
    expect(screen.getByText('Resource is required')).toBeInTheDocument();

    // assert data was not saved to local storage
    expect(
      JSON.parse(localStorage.getItem('grv_teleport_onboard_survey'))
    ).toBeNull();

    // assert data was not sent to sales center
    expect(surveyService.submitSurvey).not.toHaveBeenCalled();

    // assert prehog event was not triggered
    expect(userEventService.capturePreUserEvent).not.toHaveBeenCalled();
    expect(userEventService.captureUserEvent).not.toHaveBeenCalled();
  });

  test('submits responses in onboard mode', async () => {
    jest
      .spyOn(surveyService, 'getSurveyCompanyResults')
      .mockImplementation(() =>
        Promise.resolve({
          companyName: '',
          employeeCount: '',
          marketingParams: {
            campaign: 'c_1',
            source: 's_1',
            medium: 'm_1',
            intent: 't_i',
          },
        })
      );

    props.onboard = true;
    props.username = 'user-000';
    render(<Questionnaire {...props} />);

    await screen.findByText('Tell us about yourself');

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
        marketingParams: {
          campaign: 'c_1',
          source: 's_1',
          medium: 'm_1',
          intent: 't_i',
        },
        resources: [
          'RESOURCE_WEB_APPLICATIONS',
          'RESOURCE_WINDOWS_DESKTOPS',
          'RESOURCE_KUBERNETES',
        ],
        role: 'VP',
        team: 'LEGAL',
        username: 'user-000',
      })
    );
    localStorage.clear();

    // assert data was not sent to sales center
    expect(surveyService.submitSurvey).not.toHaveBeenCalled();

    // assert posthog event triggered
    expect(userEventService.capturePreUserEvent).toHaveBeenCalledWith({
      event: 'tp.ui.onboard.questionnaire.submit',
      username: 'user-000',
    });
    expect(userEventService.captureUserEvent).not.toHaveBeenCalled();
  });

  test('submits responses in non-onboard mode', async () => {
    jest
      .spyOn(surveyService, 'getSurveyCompanyResults')
      .mockImplementation(() =>
        Promise.resolve({
          companyName: '',
          employeeCount: '',
          marketingParams: {
            campaign: 'c_1',
            source: 's_1',
            medium: 'm_1',
            intent: 't_i',
          },
        })
      );

    props.onboard = false;
    props.username = 'user-000';
    render(<Questionnaire {...props} />);

    await screen.findByText('Tell us about yourself');

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

    // assert data is not saved to local storage
    expect(
      JSON.parse(localStorage.getItem('grv_teleport_onboard_survey'))
    ).toBeNull();
    localStorage.clear();

    // assert data was sent to sales center
    expect(surveyService.submitSurvey).toHaveBeenCalled();
    expect(surveyService.submitSurvey).toHaveBeenCalledWith(
      expect.objectContaining({
        companyName: 'Teleport',
        employeeCount: '5000+',
        resources: [
          'RESOURCE_WEB_APPLICATIONS',
          'RESOURCE_WINDOWS_DESKTOPS',
          'RESOURCE_KUBERNETES',
        ],
        role: 'VP',
        team: 'LEGAL',
        username: 'user-000',
      })
    );
    expect(api.put).toHaveBeenCalledWith(
      cfg.api.userPreferencesPath,
      expect.objectContaining({
        onboard: {
          marketingParams: {
            campaign: 'c_1',
            intent: 't_i',
            medium: 'm_1',
            source: 's_1',
          },
          preferredResources: [5, 1, 4],
        },
      })
    );

    // assert posthog event triggered
    expect(userEventService.capturePreUserEvent).not.toHaveBeenCalled();
    expect(userEventService.captureUserEvent).toHaveBeenCalledWith({
      event: 'tp.ui.onboard.questionnaire.submit',
    });
  });
});
