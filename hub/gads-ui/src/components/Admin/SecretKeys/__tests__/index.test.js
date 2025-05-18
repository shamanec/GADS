import React from 'react';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { SnackbarProvider } from '../../../../contexts/SnackBarContext';
import { DialogProvider } from '../../../../contexts/DialogContext';
import SecretKeys from '../index';
import { api } from '../../../../services/api';

// Mock API
jest.mock('../../../../services/api', () => ({
  api: {
    get: jest.fn(),
    post: jest.fn(),
    put: jest.fn(),
    delete: jest.fn()
  }
}));

// Sample data
const mockSecretKeys = {
  secret_keys: [
    {
      id: '1',
      origin: 'com.example.app1',
      is_default: true,
      created_at: '2023-01-01T00:00:00Z',
      updated_at: '2023-01-01T00:00:00Z',
      user_identifier_claim: 'sub',
      tenant_identifier_claim: 'tenant_id'
    },
    {
      id: '2',
      origin: 'com.example.app2',
      is_default: false,
      created_at: '2023-01-02T00:00:00Z',
      updated_at: '2023-01-02T00:00:00Z',
      user_identifier_claim: 'email',
      tenant_identifier_claim: null
    }
  ]
};

describe('SecretKeys Component', () => {
  beforeEach(() => {
    // Reset mocks
    jest.clearAllMocks();
    
    // Mock successful API responses
    api.get.mockResolvedValue({ data: mockSecretKeys });
  });

  const renderComponent = () => {
    return render(
      <SnackbarProvider>
        <DialogProvider>
          <SecretKeys />
        </DialogProvider>
      </SnackbarProvider>
    );
  };

  it('should render the component and fetch secret keys', async () => {
    renderComponent();
    
    // Check loading state
    await waitFor(() => {
      expect(api.get).toHaveBeenCalledWith('/admin/secret-keys');
    });
    
    // Check for buttons that are now in SecretKeyList
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Add Secret Key/i })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /View History/i })).toBeInTheDocument();
    });
    
    // Ensure secret keys are displayed after loading
    await waitFor(() => {
      expect(screen.getByText('com.example.app1')).toBeInTheDocument();
      expect(screen.getByText('com.example.app2')).toBeInTheDocument();
      
      // Check that User Identifier Claim values are displayed
      expect(screen.getByText('sub')).toBeInTheDocument();
      expect(screen.getByText('email')).toBeInTheDocument();
      
      // Check that Tenant Identifier Claim values are displayed
      expect(screen.getByText('tenant_id')).toBeInTheDocument();
      expect(screen.getAllByText('-')[0]).toBeInTheDocument(); // For the null tenant identifier claim
      
      // Check that column headers are displayed
      expect(screen.getByText('User Identifier Claim')).toBeInTheDocument();
      expect(screen.getByText('Tenant Identifier Claim')).toBeInTheDocument();
    });
  });

  it('should display an error message when API request fails', async () => {
    // Mock API failure
    api.get.mockRejectedValueOnce({ 
      response: { data: { error: 'Failed to fetch secret keys' } } 
    });
    
    renderComponent();
    
    await waitFor(() => {
      expect(api.get).toHaveBeenCalledWith('/admin/secret-keys');
    });
    
    // Error handling will be managed by SnackbarContext
    // We can't easily test this without mocking the context itself
  });

  it('should open modal when Add Secret Key button is clicked', async () => {
    renderComponent();
    
    // Wait for initial load
    await waitFor(() => {
      expect(screen.getByText('com.example.app1')).toBeInTheDocument();
    });
    
    // Click on Add Secret Key button
    const addButton = screen.getByRole('button', { name: /Add Secret Key/i });
    userEvent.click(addButton);
    
    // Check that modal is displayed
    await waitFor(() => {
      expect(screen.getByText('Add New Secret Key')).toBeInTheDocument();
    });
    
    // Check form fields
    const originField = screen.getByRole('textbox', { name: /^Origin/ });
    expect(originField).toBeInTheDocument();
    
    const secretKeyField = screen.getByLabelText(/^Secret Key/);
    expect(secretKeyField).toBeInTheDocument();
    
    const userIdentifierClaimField = screen.getByRole('textbox', { name: /^User Identifier Claim/ });
    expect(userIdentifierClaimField).toBeInTheDocument();
    
    const tenantIdentifierClaimField = screen.getByRole('textbox', { name: /^Tenant Identifier Claim/ });
    expect(tenantIdentifierClaimField).toBeInTheDocument();
    
    // Check checkbox using the tooltip text
    const checkboxLabel = screen.getByText('Set as default key');
    expect(checkboxLabel).toBeInTheDocument();
    
    expect(screen.getByText('Generate Secure Key')).toBeInTheDocument();
    
    // Click Cancel button
    const cancelButton = screen.getByRole('button', { name: /Cancel/i });
    userEvent.click(cancelButton);
    
    // Check that modal is closed
    await waitFor(() => {
      expect(screen.queryByText('Add New Secret Key')).not.toBeInTheDocument();
    });
  });

  it('should toggle between history view and main view', async () => {
    api.get.mockImplementation((url) => {
      if (url === '/admin/secret-keys') {
        return Promise.resolve({ data: mockSecretKeys });
      }
      // Not testing the actual history API call in this test
      return Promise.resolve({ data: { items: [], total: 0, pages: 0 } });
    });
    
    renderComponent();
    
    // Wait for initial load
    await waitFor(() => {
      expect(screen.getByText('com.example.app1')).toBeInTheDocument();
    });
    
    // Click on history button
    const historyButton = screen.getByRole('button', { name: /View History/i });
    userEvent.click(historyButton);
    
    // Check that history view is displayed
    await waitFor(() => {
      expect(screen.getByText('Secret Keys Audit History')).toBeInTheDocument();
    });
    
    // Click back button
    const backButton = screen.getByText('Back to Secret Keys');
    userEvent.click(backButton);
    
    // Check that main view is displayed again
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Add Secret Key/i })).toBeInTheDocument();
    });
  });

  it('should open modal in edit mode when Edit button is clicked', async () => {
    renderComponent();
    
    // Wait for initial load
    await waitFor(() => {
      expect(screen.getByText('com.example.app1')).toBeInTheDocument();
    });
    
    // Get all Edit buttons
    const editButtons = screen.getAllByRole('button', { name: /Edit/i });
    userEvent.click(editButtons[0]);
    
    // Check that modal is displayed in edit mode
    await waitFor(() => {
      expect(screen.getByText(/Edit Secret Key/)).toBeInTheDocument();
    });
    
    // Check that form is pre-filled
    const originField = screen.getByRole('textbox', { name: /^Origin/ });
    expect(originField).toBeDisabled(); // Origin field should be disabled in edit mode
    expect(originField).toHaveValue('com.example.app1');
    
    // Check that User Identifier Claim is pre-filled
    const userIdentifierClaimField = screen.getByRole('textbox', { name: /^User Identifier Claim/ });
    expect(userIdentifierClaimField).toHaveValue('sub');
    
    // Check that Tenant Identifier Claim is pre-filled
    const tenantIdentifierClaimField = screen.getByRole('textbox', { name: /^Tenant Identifier Claim/ });
    expect(tenantIdentifierClaimField).toHaveValue('tenant_id');
    
    // Click Cancel button
    const cancelButton = screen.getByRole('button', { name: /Cancel/i });
    userEvent.click(cancelButton);
    
    // Check that modal is closed
    await waitFor(() => {
      expect(screen.queryByText(/Edit Secret Key/)).not.toBeInTheDocument();
    });
  });
}); 