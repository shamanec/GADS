import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { SnackbarProvider } from '../../../../contexts/SnackBarContext';
import { DialogProvider } from '../../../../contexts/DialogContext';
import SecretKeyList from '../SecretKeyList';
import { api } from '../../../../services/api';

// Mock API
jest.mock('../../../../services/api', () => ({
  api: {
    post: jest.fn(),
  }
}));

// Sample data
const mockSecretKeys = [
  {
    id: '1',
    origin: 'com.example.app1',
    secret_key: 'secret1',
    is_default: true,
    active: true,
    created_at: '2023-01-01T00:00:00Z'
  },
  {
    id: '2',
    origin: 'com.example.app2',
    secret_key: 'secret2',
    is_default: false,
    active: true,
    created_at: '2023-01-02T00:00:00Z'
  },
  {
    id: '3',
    origin: 'com.example.app3',
    secret_key: 'secret3',
    is_default: false,
    active: false,
    created_at: '2023-01-03T00:00:00Z'
  }
];

describe('SecretKeyList Component', () => {
  beforeEach(() => {
    // Reset mocks
    jest.clearAllMocks();
    
    // Mock successful API responses
    api.post.mockResolvedValue({ data: { success: true } });
  });

  const renderComponent = (props = {}) => {
    const defaultProps = {
      secretKeys: mockSecretKeys,
      loading: false,
      onReload: jest.fn(),
      onViewHistory: jest.fn(),
      ...props
    };
    
    return render(
      <SnackbarProvider>
        <DialogProvider>
          <SecretKeyList {...defaultProps} />
        </DialogProvider>
      </SnackbarProvider>
    );
  };

  it('should render the list of secret keys', () => {
    renderComponent();
    
    // Check table headers
    expect(screen.getByText('Origin')).toBeInTheDocument();
    expect(screen.getByText('Created At')).toBeInTheDocument();
    expect(screen.getByText('Updated At')).toBeInTheDocument();
    expect(screen.getByText('Actions')).toBeInTheDocument();
    
    // Check origins are displayed
    expect(screen.getByText('com.example.app1')).toBeInTheDocument();
    expect(screen.getByText('com.example.app2')).toBeInTheDocument();
    expect(screen.getByText('com.example.app3')).toBeInTheDocument();
    
    // Check default badge
    expect(screen.getByText('Default')).toBeInTheDocument();
  });

  it('should show loading state', () => {
    renderComponent({ loading: true });
    
    // Check for loading spinner
    expect(screen.getByRole('progressbar')).toBeInTheDocument();
    
    // Table content should not be rendered
    expect(screen.queryByText('com.example.app1')).not.toBeInTheDocument();
  });

  it('should show empty state when no keys are available', () => {
    renderComponent({ secretKeys: [] });
    
    // Check for empty state message
    expect(screen.getByText('No secret keys found. Add your first secret key using the form above.')).toBeInTheDocument();
  });

  it('should filter secret keys by origin', async () => {
    renderComponent();
    
    // All origins should be visible initially
    expect(screen.getByText('com.example.app1')).toBeInTheDocument();
    expect(screen.getByText('com.example.app2')).toBeInTheDocument();
    expect(screen.getByText('com.example.app3')).toBeInTheDocument();
    
    // Type in filter
    const filterInput = screen.getByPlaceholderText('Filter by origin');
    await userEvent.type(filterInput, 'app2');
    
    // Only app2 should be visible
    expect(screen.queryByText('com.example.app1')).not.toBeInTheDocument();
    expect(screen.getByText('com.example.app2')).toBeInTheDocument();
    expect(screen.queryByText('com.example.app3')).not.toBeInTheDocument();
  });

  it('should call onViewHistory when view history button is clicked', async () => {
    const onViewHistoryMock = jest.fn();
    renderComponent({ onViewHistory: onViewHistoryMock });
    
    // Click on view history button
    const viewHistoryButton = screen.getByText('View History');
    await userEvent.click(viewHistoryButton);
    
    // Verify callback was called
    expect(onViewHistoryMock).toHaveBeenCalledTimes(1);
  });

  it('should show edit form when edit button is clicked', async () => {
    renderComponent();
    
    // Initially, we should see the key list
    expect(screen.getByText('com.example.app1')).toBeInTheDocument();
    
    // Click on edit button for the first key
    const editButtons = screen.getAllByLabelText('Edit', { selector: 'button' });
    await userEvent.click(editButtons[0]);
    
    // We should now see the edit form
    expect(screen.getByText('Edit Secret Key')).toBeInTheDocument();
    
    // Original key list should not be visible
    expect(screen.queryByText('Secret Key')).not.toBeInTheDocument();
  });

  it('should prevent disabling the default key', async () => {
    const { container } = renderComponent();
    
    // Find disable buttons
    const disableButtons = screen.getAllByLabelText('Disable', { selector: 'button' });
    
    // Try to disable the default key (first key in our mock data)
    await userEvent.click(disableButtons[0]);
    
    // We can't directly test the dialog appearance without mocking the dialog context
    // This would typically show a dialog with the message "Cannot Disable Default Key"
    
    // Verify that the API call to disable the key was not made
    expect(api.post).not.toHaveBeenCalled();
  });
}); 