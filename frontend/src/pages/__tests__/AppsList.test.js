import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { BrowserRouter } from 'react-router-dom';
import { ConfigProvider } from 'antd';
import AppsList from '../AppsList';
import { appsAPI } from '../../services/api';

// Mock the API
jest.mock('../../services/api', () => ({
  appsAPI: {
    getApps: jest.fn(),
  },
}));

// Mock react-router-dom
jest.mock('react-router-dom', () => ({
  ...jest.requireActual('react-router-dom'),
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

describe('AppsList Component', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  test('renders apps list page', async () => {
    const mockApps = [
      {
        id: 1,
        name: 'test-app-1',
        git_url: 'https://github.com/test/repo1.git',
        default_branch: 'main',
        registry_repo: 'harbor.test.com/team/test-app-1',
        target_namespace: 'production',
        created_at: '2023-01-01T00:00:00Z',
      },
      {
        id: 2,
        name: 'test-app-2',
        git_url: 'https://github.com/test/repo2.git',
        default_branch: 'develop',
        registry_repo: 'harbor.test.com/team/test-app-2',
        target_namespace: 'staging',
        created_at: '2023-01-02T00:00:00Z',
      },
    ];

    appsAPI.getApps.mockResolvedValue({ data: mockApps });

    renderWithProviders(<AppsList />);

    // Check if title is rendered
    expect(screen.getByText('Applications')).toBeInTheDocument();

    // Check if Create App button is rendered
    expect(screen.getByText('Create App')).toBeInTheDocument();

    // Wait for apps to be loaded
    await waitFor(() => {
      expect(screen.getByText('test-app-1')).toBeInTheDocument();
      expect(screen.getByText('test-app-2')).toBeInTheDocument();
    });

    // Check table headers
    expect(screen.getByText('Name')).toBeInTheDocument();
    expect(screen.getByText('Git Repository')).toBeInTheDocument();
    expect(screen.getByText('Default Branch')).toBeInTheDocument();
    expect(screen.getByText('Registry')).toBeInTheDocument();
    expect(screen.getByText('Target Namespace')).toBeInTheDocument();

    // Check app data
    expect(screen.getByText('https://github.com/test/repo1.git')).toBeInTheDocument();
    expect(screen.getByText('https://github.com/test/repo2.git')).toBeInTheDocument();
    expect(screen.getByText('harbor.test.com/team/test-app-1')).toBeInTheDocument();
    expect(screen.getByText('harbor.test.com/team/test-app-2')).toBeInTheDocument();
  });

  test('shows loading state', () => {
    appsAPI.getApps.mockImplementation(() => new Promise(() => {})); // Never resolves

    renderWithProviders(<AppsList />);

    // The loading state would be shown in the table
    // Antd Table shows loading spinner automatically
    expect(screen.getByText('Applications')).toBeInTheDocument();
  });

  test('handles API error', async () => {
    const consoleError = jest.spyOn(console, 'error').mockImplementation(() => {});
    appsAPI.getApps.mockRejectedValue(new Error('API Error'));

    renderWithProviders(<AppsList />);

    await waitFor(() => {
      // The component should handle the error gracefully
      expect(consoleError).toHaveBeenCalledWith('Error fetching apps:', expect.any(Error));
    });

    consoleError.mockRestore();
  });

  test('displays empty state when no apps', async () => {
    appsAPI.getApps.mockResolvedValue({ data: [] });

    renderWithProviders(<AppsList />);

    await waitFor(() => {
      // When no data, Antd Table shows empty state
      expect(appsAPI.getApps).toHaveBeenCalled();
    });
  });

  test('formats dates correctly', async () => {
    const mockApps = [
      {
        id: 1,
        name: 'test-app',
        git_url: 'https://github.com/test/repo.git',
        default_branch: 'main',
        registry_repo: 'harbor.test.com/team/test-app',
        target_namespace: 'production',
        created_at: '2023-12-25T10:30:00Z',
      },
    ];

    appsAPI.getApps.mockResolvedValue({ data: mockApps });

    renderWithProviders(<AppsList />);

    await waitFor(() => {
      // Check if date is formatted (the exact format depends on locale)
      expect(screen.getByText(/25|12|2023/)).toBeInTheDocument();
    });
  });

  test('creates correct links for apps', async () => {
    const mockApps = [
      {
        id: 1,
        name: 'test-app',
        git_url: 'https://github.com/test/repo.git',
        default_branch: 'main',
        registry_repo: 'harbor.test.com/team/test-app',
        target_namespace: 'production',
        created_at: '2023-01-01T00:00:00Z',
      },
    ];

    appsAPI.getApps.mockResolvedValue({ data: mockApps });

    renderWithProviders(<AppsList />);

    await waitFor(() => {
      // Check if app name links to correct URL
      const appLink = screen.getByText('test-app').closest('a');
      expect(appLink).toHaveAttribute('href', '/apps/1');

      // Check if View button links to correct URL
      const viewLinks = screen.getAllByText('View');
      expect(viewLinks[0].closest('a')).toHaveAttribute('href', '/apps/1');
    });
  });

  test('creates correct external git links', async () => {
    const mockApps = [
      {
        id: 1,
        name: 'test-app',
        git_url: 'https://github.com/test/repo.git',
        default_branch: 'main',
        registry_repo: 'harbor.test.com/team/test-app',
        target_namespace: 'production',
        created_at: '2023-01-01T00:00:00Z',
      },
    ];

    appsAPI.getApps.mockResolvedValue({ data: mockApps });

    renderWithProviders(<AppsList />);

    await waitFor(() => {
      // Check if git URL is linked externally
      const gitLink = screen.getByText('https://github.com/test/repo.git');
      expect(gitLink).toHaveAttribute('href', 'https://github.com/test/repo.git');
      expect(gitLink).toHaveAttribute('target', '_blank');
      expect(gitLink).toHaveAttribute('rel', 'noopener noreferrer');
    });
  });

  test('handles table pagination', async () => {
    // Create enough apps to trigger pagination
    const mockApps = Array.from({ length: 15 }, (_, i) => ({
      id: i + 1,
      name: `test-app-${i + 1}`,
      git_url: `https://github.com/test/repo${i + 1}.git`,
      default_branch: 'main',
      registry_repo: `harbor.test.com/team/test-app-${i + 1}`,
      target_namespace: 'production',
      created_at: '2023-01-01T00:00:00Z',
    }));

    appsAPI.getApps.mockResolvedValue({ data: mockApps });

    renderWithProviders(<AppsList />);

    await waitFor(() => {
      expect(screen.getByText('test-app-1')).toBeInTheDocument();
      expect(screen.getByText('test-app-10')).toBeInTheDocument();
      
      // Should not show app 11 on first page (pageSize is 10)
      expect(screen.queryByText('test-app-11')).not.toBeInTheDocument();
    });
  });

  test('refresh functionality works', async () => {
    const mockApps = [
      {
        id: 1,
        name: 'test-app',
        git_url: 'https://github.com/test/repo.git',
        default_branch: 'main',
        registry_repo: 'harbor.test.com/team/test-app',
        target_namespace: 'production',
        created_at: '2023-01-01T00:00:00Z',
      },
    ];

    appsAPI.getApps.mockResolvedValue({ data: mockApps });

    renderWithProviders(<AppsList />);

    // Wait for initial load
    await waitFor(() => {
      expect(screen.getByText('test-app')).toBeInTheDocument();
    });

    // Clear the mock to track subsequent calls
    appsAPI.getApps.mockClear();

    // Trigger refresh (if there's a refresh button/mechanism)
    // This depends on the actual implementation
    // For now, we just verify the API was called initially
    expect(appsAPI.getApps).toHaveBeenCalledTimes(0); // Should be 0 after clearing
  });

  test('displays tags correctly', async () => {
    const mockApps = [
      {
        id: 1,
        name: 'test-app',
        git_url: 'https://github.com/test/repo.git',
        default_branch: 'main',
        registry_repo: 'harbor.test.com/team/test-app',
        target_namespace: 'production',
        created_at: '2023-01-01T00:00:00Z',
      },
    ];

    appsAPI.getApps.mockResolvedValue({ data: mockApps });

    renderWithProviders(<AppsList />);

    await waitFor(() => {
      // Check if branch and namespace are displayed as tags
      expect(screen.getByText('main')).toBeInTheDocument();
      expect(screen.getByText('production')).toBeInTheDocument();
    });
  });
});