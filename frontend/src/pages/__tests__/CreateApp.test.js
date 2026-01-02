import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BrowserRouter } from 'react-router-dom';
import { ConfigProvider } from 'antd';
import CreateApp from '../CreateApp';
import { appsAPI } from '../../services/api';

// Mock the API
jest.mock('../../services/api', () => ({
  appsAPI: {
    createApp: jest.fn(),
  },
}));

// Mock react-router-dom
const mockNavigate = jest.fn();
jest.mock('react-router-dom', () => ({
  ...jest.requireActual('react-router-dom'),
  useNavigate: () => mockNavigate,
  Link: ({ children, to, ...props }) => <a href={to} {...props}>{children}</a>,
}));

const renderWithProviders = (component) => {
  return render(
    <BrowserRouter>
      <ConfigProvider>
        {component}
      </ConfigProvider>
    </BrowserRouter>
  );
};

describe('CreateApp Component', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  test('renders create app form', () => {
    renderWithProviders(<CreateApp />);

    expect(screen.getByText('Create Application')).toBeInTheDocument();
    expect(screen.getByLabelText('Application Name')).toBeInTheDocument();
    expect(screen.getByLabelText('Git Repository URL')).toBeInTheDocument();
    expect(screen.getByLabelText('Default Branch')).toBeInTheDocument();
    expect(screen.getByLabelText('Dockerfile Path')).toBeInTheDocument();
    expect(screen.getByLabelText('Build Context Path')).toBeInTheDocument();
    expect(screen.getByLabelText('Container Registry Repository')).toBeInTheDocument();
    expect(screen.getByLabelText('Target Kubernetes Namespace')).toBeInTheDocument();
    expect(screen.getByLabelText('Target Deployment Name')).toBeInTheDocument();
    
    expect(screen.getByRole('button', { name: 'Create Application' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
  });

  test('has correct default values', () => {
    renderWithProviders(<CreateApp />);

    expect(screen.getByDisplayValue('main')).toBeInTheDocument();
    expect(screen.getByDisplayValue('Dockerfile')).toBeInTheDocument();
    expect(screen.getByDisplayValue('.')).toBeInTheDocument();
  });

  test('validates required fields', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateApp />);

    // Try to submit form without filling required fields
    const submitButton = screen.getByRole('button', { name: 'Create Application' });
    await user.click(submitButton);

    // Form should show validation errors
    await waitFor(() => {
      expect(screen.getByText('Please enter application name')).toBeInTheDocument();
      expect(screen.getByText('Please enter Git repository URL')).toBeInTheDocument();
      expect(screen.getByText('Please enter registry repository')).toBeInTheDocument();
      expect(screen.getByText('Please enter target namespace')).toBeInTheDocument();
      expect(screen.getByText('Please enter deployment name')).toBeInTheDocument();
    });
  });

  test('validates application name pattern', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateApp />);

    const nameInput = screen.getByLabelText('Application Name');
    
    // Enter invalid name (uppercase letters)
    await user.type(nameInput, 'Invalid-Name-123');
    
    const submitButton = screen.getByRole('button', { name: 'Create Application' });
    await user.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Only lowercase letters, numbers, and hyphens allowed')).toBeInTheDocument();
    });
  });

  test('validates Git URL format', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateApp />);

    const urlInput = screen.getByLabelText('Git Repository URL');
    
    // Enter invalid URL
    await user.type(urlInput, 'not-a-url');
    
    const submitButton = screen.getByRole('button', { name: 'Create Application' });
    await user.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText('Please enter a valid URL')).toBeInTheDocument();
    });
  });

  test('successfully creates app', async () => {
    const user = userEvent.setup();
    const mockResponse = { data: { id: 1, name: 'test-app' } };
    appsAPI.createApp.mockResolvedValue(mockResponse);

    renderWithProviders(<CreateApp />);

    // Fill out form
    await user.type(screen.getByLabelText('Application Name'), 'test-app');
    await user.type(screen.getByLabelText('Git Repository URL'), 'https://github.com/test/repo.git');
    await user.type(screen.getByLabelText('Container Registry Repository'), 'harbor.test.com/team/test-app');
    await user.type(screen.getByLabelText('Target Kubernetes Namespace'), 'production');
    await user.type(screen.getByLabelText('Target Deployment Name'), 'test-app');

    // Submit form
    const submitButton = screen.getByRole('button', { name: 'Create Application' });
    await user.click(submitButton);

    await waitFor(() => {
      expect(appsAPI.createApp).toHaveBeenCalledWith({
        name: 'test-app',
        git_url: 'https://github.com/test/repo.git',
        default_branch: 'main',
        dockerfile_path: 'Dockerfile',
        context_path: '.',
        registry_repo: 'harbor.test.com/team/test-app',
        target_namespace: 'production',
        target_deploy_name: 'test-app',
        git_secret_ref: '',
        registry_secret_ref: '',
      });

      expect(mockNavigate).toHaveBeenCalledWith('/apps/1');
    });
  });

  test('handles API error during creation', async () => {
    const user = userEvent.setup();
    const consoleError = jest.spyOn(console, 'error').mockImplementation(() => {});
    appsAPI.createApp.mockRejectedValue(new Error('API Error'));

    renderWithProviders(<CreateApp />);

    // Fill out form with minimum required fields
    await user.type(screen.getByLabelText('Application Name'), 'test-app');
    await user.type(screen.getByLabelText('Git Repository URL'), 'https://github.com/test/repo.git');
    await user.type(screen.getByLabelText('Container Registry Repository'), 'harbor.test.com/team/test-app');
    await user.type(screen.getByLabelText('Target Kubernetes Namespace'), 'production');
    await user.type(screen.getByLabelText('Target Deployment Name'), 'test-app');

    // Submit form
    const submitButton = screen.getByRole('button', { name: 'Create Application' });
    await user.click(submitButton);

    await waitFor(() => {
      expect(consoleError).toHaveBeenCalledWith('Error creating app:', expect.any(Error));
    });

    consoleError.mockRestore();
  });

  test('shows loading state during submission', async () => {
    const user = userEvent.setup();
    let resolvePromise;
    appsAPI.createApp.mockImplementation(() => new Promise(resolve => {
      resolvePromise = resolve;
    }));

    renderWithProviders(<CreateApp />);

    // Fill out form
    await user.type(screen.getByLabelText('Application Name'), 'test-app');
    await user.type(screen.getByLabelText('Git Repository URL'), 'https://github.com/test/repo.git');
    await user.type(screen.getByLabelText('Container Registry Repository'), 'harbor.test.com/team/test-app');
    await user.type(screen.getByLabelText('Target Kubernetes Namespace'), 'production');
    await user.type(screen.getByLabelText('Target Deployment Name'), 'test-app');

    // Submit form
    const submitButton = screen.getByRole('button', { name: 'Create Application' });
    await user.click(submitButton);

    // Button should be disabled and show loading state
    expect(submitButton).toBeDisabled();

    // Resolve the promise to clean up
    resolvePromise({ data: { id: 1 } });
  });

  test('cancel button navigates back', () => {
    renderWithProviders(<CreateApp />);

    const cancelButton = screen.getByRole('button', { name: 'Cancel' });
    expect(cancelButton.closest('a')).toHaveAttribute('href', '/apps');
  });

  test('back arrow navigates to apps list', () => {
    renderWithProviders(<CreateApp />);

    const backButton = screen.getByRole('button');
    expect(backButton.closest('a')).toHaveAttribute('href', '/apps');
  });

  test('fills form with custom values', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateApp />);

    // Fill all fields with custom values
    await user.clear(screen.getByLabelText('Default Branch'));
    await user.type(screen.getByLabelText('Default Branch'), 'develop');
    
    await user.clear(screen.getByLabelText('Dockerfile Path'));
    await user.type(screen.getByLabelText('Dockerfile Path'), 'docker/Dockerfile');
    
    await user.clear(screen.getByLabelText('Build Context Path'));
    await user.type(screen.getByLabelText('Build Context Path'), 'src');

    await user.type(screen.getByLabelText('Git Secret Reference (Optional)'), 'git-credentials');
    await user.type(screen.getByLabelText('Registry Secret Reference (Optional)'), 'registry-credentials');

    expect(screen.getByDisplayValue('develop')).toBeInTheDocument();
    expect(screen.getByDisplayValue('docker/Dockerfile')).toBeInTheDocument();
    expect(screen.getByDisplayValue('src')).toBeInTheDocument();
    expect(screen.getByDisplayValue('git-credentials')).toBeInTheDocument();
    expect(screen.getByDisplayValue('registry-credentials')).toBeInTheDocument();
  });

  test('displays helpful tooltips', () => {
    renderWithProviders(<CreateApp />);

    // Check for tooltip text
    expect(screen.getByText('Path to Dockerfile relative to repository root')).toBeInTheDocument();
    expect(screen.getByText('Build context directory relative to repository root')).toBeInTheDocument();
    expect(screen.getByText('Kubernetes secret containing Git credentials')).toBeInTheDocument();
    expect(screen.getByText('Kubernetes secret containing registry credentials')).toBeInTheDocument();
  });

  test('form layout and spacing', () => {
    renderWithProviders(<CreateApp />);

    // Check that form uses vertical layout
    const form = screen.getByRole('form');
    expect(form).toBeInTheDocument();

    // Check that all required fields are marked
    const requiredFields = [
      'Application Name',
      'Git Repository URL', 
      'Container Registry Repository',
      'Target Kubernetes Namespace',
      'Target Deployment Name'
    ];

    requiredFields.forEach(field => {
      const input = screen.getByLabelText(field);
      expect(input).toBeRequired();
    });
  });
});