import React from 'react';
import { fireEvent, render, screen, userEvent } from 'design/utils/testing';

import { Questionnaire } from './Questionnaire';

describe('questionnaire', () => {
  let spyFetch;
  beforeEach(() => {
    spyFetch = jest.spyOn(global, 'fetch').mockResolvedValue(null);
  });

  afterEach(() => jest.resetAllMocks());

  test('loads each question and expected', async () => {
    const mockSubmit = jest.fn();
    render(<Questionnaire onSubmit={mockSubmit} />);

    expect(screen.getByText('Tell us about yourself')).toBeInTheDocument();
    expect(screen.getByLabelText('Company Name')).toBeInTheDocument();
    expect(screen.getByLabelText('Number of Employees')).toBeInTheDocument();
    expect(screen.getByLabelText('Which Team are you on?')).toBeInTheDocument();
    expect(screen.getByLabelText('Job Title')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Which infrastructure resources do you need to access frequently?'
      )
    ).toBeInTheDocument();

    expect(screen.getByRole('button', { name: /submit/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /skip/i })).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: /skip/i }));
    expect(global.fetch).not.toHaveBeenCalled();
    expect(mockSubmit).toHaveBeenCalledTimes(1);
  });

  test('shows validation errors', async () => {
    const mockSubmit = jest.fn();
    render(<Questionnaire onSubmit={mockSubmit} />);

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

    expect(global.fetch).not.toHaveBeenCalled();
    expect(mockSubmit).not.toHaveBeenCalled();
  });

  test('form submission', async () => {
    const mockSubmit = jest.fn();
    render(<Questionnaire onSubmit={mockSubmit} />);

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

    await userEvent.click(screen.getByRole('button', { name: /Submit/i }));
    expect(mockSubmit).toHaveBeenCalledTimes(1);

    const formData = new FormData();
    formData.set('title', 'VP');
    formData.set('teamsize', '5000+');
    formData.set('team', 'LEGAL');
    formData.set('company', 'Teleport');
    formData.set(
      'access_needs',
      JSON.stringify(['RESOURCE_WEB_APPLICATIONS', 'RESOURCE_WINDOWS_DESKTOPS'])
    );

    expect(global.fetch).toHaveBeenCalledTimes(1);
    const sentFormData = spyFetch.mock.calls[0][1].body;
    const gotFormData = Object.fromEntries(sentFormData.entries());

    expect(Object.fromEntries(formData.entries())).toEqual(gotFormData);
  });
});
