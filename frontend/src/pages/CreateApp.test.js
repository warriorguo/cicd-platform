import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import CreateApp from './CreateApp';
import { appsAPI } from '../services/api';

// Mock the API
jest.mock('../services/api');

// Mock react-router-dom navigate
const mockNavigate = jest.fn();
jest.mock('react-router-dom', () => ({
  ...jest.requireActual('react-router-dom'),
  useNavigate: () => mockNavigate,
}));

describe('CreateApp Component', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  const renderCreateApp = () => {
    return render(
      <BrowserRouter>
        <CreateApp />
      </BrowserRouter>
    );
  };

  test('renders CreateApp form', () => {
    renderCreateApp();
    
    expect(screen.getByText('Create Application')).toBeInTheDocument();
    expect(screen.getByLabelText(/Application Name/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/Git Repository URL/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/Build Type/i)).toBeInTheDocument();
  });

  test('has default values', () => {
    renderCreateApp();
    
    expect(screen.getByDisplayValue('main')).toBeInTheDocument();
    expect(screen.getByDisplayValue('Dockerfile')).toBeInTheDocument();
    expect(screen.getByDisplayValue('.')).toBeInTheDocument();
  });

  test('shows Dockerfile build type option', () => {
    renderCreateApp();
    
    const buildTypeSelect = screen.getByLabelText(/Build Type/i);
    fireEvent.mouseDown(buildTypeSelect);
    
    expect(screen.getByText('Dockerfile')).toBeInTheDocument();
    expect(screen.getByText('Docker Compose')).toBeInTheDocument();
  });

  test('changes dockerfile path when build type changes to docker-compose', async () => {
    renderCreateApp();
    
    const buildTypeSelect = screen.getByLabelText(/Build Type/i);
    
    // Open dropdown
    fireEvent.mouseDown(buildTypeSelect);
    
    // Select Docker Compose
    const dockerComposeOption = screen.getByText('Docker Compose');
    fireEvent.click(dockerComposeOption);
    
    // Wait for form update
    await waitFor(() => {
      const dockerfilePathInput = screen.getByDisplayValue('docker-compose.yml');
      expect(dockerfilePathInput).toBeInTheDocument();
    });
  });

  test('changes dockerfile path back when build type changes to dockerfile', async () => {
    renderCreateApp();
    
    const buildTypeSelect = screen.getByLabelText(/Build Type/i);
    
    // Change to Docker Compose
    fireEvent.mouseDown(buildTypeSelect);
    fireEvent.click(screen.getByText('Docker Compose'));
    
    await waitFor(() => {
      expect(screen.getByDisplayValue('docker-compose.yml')).toBeInTheDocument();
    });
    
    // Change back to Dockerfile
    fireEvent.mouseDown(buildTypeSelect);
    fireEvent.click(screen.getByText('Dockerfile'));
    
    await waitFor(() => {
      expect(screen.getByDisplayValue('Dockerfile')).toBeInTheDocument();
    });
  });

  test('updates field label when build type changes', async () => {
    renderCreateApp();
    
    // Initially should show "Dockerfile Path"
    expect(screen.getByLabelText(/Dockerfile Path/i)).toBeInTheDocument();
    
    const buildTypeSelect = screen.getByLabelText(/Build Type/i);
    fireEvent.mouseDown(buildTypeSelect);
    fireEvent.click(screen.getByText('Docker Compose'));
    
    // Should now show "Docker Compose File Path"
    await waitFor(() => {
      expect(screen.getByLabelText(/Docker Compose File Path/i)).toBeInTheDocument();
    });
  });

  test('submits form with dockerfile build type', async () => {
    appsAPI.createApp.mockResolvedValue({ data: { id: 1 } });
    
    renderCreateApp();
    
    // Fill in the form
    fireEvent.change(screen.getByLabelText(/Application Name/i), {
      target: { value: 'test-dockerfile-app' },
    });
    fireEvent.change(screen.getByLabelText(/Git Repository URL/i), {
      target: { value: 'https://github.com/test/repo.git' },
    });
    fireEvent.change(screen.getByLabelText(/Container Registry Repository/i), {
      target: { value: 'harbor.test.com/team/app' },
    });
    fireEvent.change(screen.getByLabelText(/Target Kubernetes Namespace/i), {
      target: { value: 'production' },
    });
    fireEvent.change(screen.getByLabelText(/Target Deployment Name/i), {
      target: { value: 'test-app' },
    });
    
    // Submit form
    const submitButton = screen.getByRole('button', { name: /Create Application/i });
    fireEvent.click(submitButton);
    
    await waitFor(() => {
      expect(appsAPI.createApp).toHaveBeenCalledWith({
        name: 'test-dockerfile-app',
        git_url: 'https://github.com/test/repo.git',
        default_branch: 'main',
        build_type: 'dockerfile',
        dockerfile_path: 'Dockerfile',
        context_path: '.',
        registry_repo: 'harbor.test.com/team/app',
        target_namespace: 'production',
        target_deploy_name: 'test-app',
        git_secret_ref: '',
        registry_secret_ref: '',
      });
    });
  });

  test('submits form with docker-compose build type', async () => {
    appsAPI.createApp.mockResolvedValue({ data: { id: 2 } });
    
    renderCreateApp();
    
    // Fill in the form
    fireEvent.change(screen.getByLabelText(/Application Name/i), {
      target: { value: 'test-compose-app' },
    });
    fireEvent.change(screen.getByLabelText(/Git Repository URL/i), {
      target: { value: 'https://github.com/test/compose.git' },
    });
    
    // Change build type to docker-compose
    const buildTypeSelect = screen.getByLabelText(/Build Type/i);
    fireEvent.mouseDown(buildTypeSelect);
    fireEvent.click(screen.getByText('Docker Compose'));
    
    fireEvent.change(screen.getByLabelText(/Container Registry Repository/i), {
      target: { value: 'harbor.test.com/team/compose-app' },
    });
    fireEvent.change(screen.getByLabelText(/Target Kubernetes Namespace/i), {
      target: { value: 'staging' },
    });
    fireEvent.change(screen.getByLabelText(/Target Deployment Name/i), {
      target: { value: 'compose-app' },
    });
    
    // Submit form
    const submitButton = screen.getByRole('button', { name: /Create Application/i });
    fireEvent.click(submitButton);
    
    await waitFor(() => {
      expect(appsAPI.createApp).toHaveBeenCalledWith({
        name: 'test-compose-app',
        git_url: 'https://github.com/test/compose.git',
        default_branch: 'main',
        build_type: 'docker-compose',
        dockerfile_path: 'docker-compose.yml',
        context_path: '.',
        registry_repo: 'harbor.test.com/team/compose-app',
        target_namespace: 'staging',
        target_deploy_name: 'compose-app',
        git_secret_ref: '',
        registry_secret_ref: '',
      });
    });
  });

  test('navigates to app detail page after successful creation', async () => {
    appsAPI.createApp.mockResolvedValue({ data: { id: 123 } });
    
    renderCreateApp();
    
    // Fill minimum required fields
    fireEvent.change(screen.getByLabelText(/Application Name/i), {
      target: { value: 'test-app' },
    });
    fireEvent.change(screen.getByLabelText(/Git Repository URL/i), {
      target: { value: 'https://github.com/test/repo.git' },
    });
    fireEvent.change(screen.getByLabelText(/Container Registry Repository/i), {
      target: { value: 'harbor.test.com/team/app' },
    });
    fireEvent.change(screen.getByLabelText(/Target Kubernetes Namespace/i), {
      target: { value: 'production' },
    });
    fireEvent.change(screen.getByLabelText(/Target Deployment Name/i), {
      target: { value: 'test-app' },
    });
    
    // Submit
    const submitButton = screen.getByRole('button', { name: /Create Application/i });
    fireEvent.click(submitButton);
    
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/apps/123');
    });
  });

  test('validates required fields', async () => {
    renderCreateApp();
    
    // Try to submit without filling required fields
    const submitButton = screen.getByRole('button', { name: /Create Application/i });
    fireEvent.click(submitButton);
    
    // API should not be called
    expect(appsAPI.createApp).not.toHaveBeenCalled();
  });

  test('shows error message on API failure', async () => {
    appsAPI.createApp.mockRejectedValue(new Error('API Error'));
    
    renderCreateApp();
    
    // Fill in form
    fireEvent.change(screen.getByLabelText(/Application Name/i), {
      target: { value: 'test-app' },
    });
    fireEvent.change(screen.getByLabelText(/Git Repository URL/i), {
      target: { value: 'https://github.com/test/repo.git' },
    });
    fireEvent.change(screen.getByLabelText(/Container Registry Repository/i), {
      target: { value: 'harbor.test.com/team/app' },
    });
    fireEvent.change(screen.getByLabelText(/Target Kubernetes Namespace/i), {
      target: { value: 'production' },
    });
    fireEvent.change(screen.getByLabelText(/Target Deployment Name/i), {
      target: { value: 'test-app' },
    });
    
    // Submit
    const submitButton = screen.getByRole('button', { name: /Create Application/i });
    fireEvent.click(submitButton);
    
    // Should not navigate on error
    await waitFor(() => {
      expect(mockNavigate).not.toHaveBeenCalled();
    });
  });
});
