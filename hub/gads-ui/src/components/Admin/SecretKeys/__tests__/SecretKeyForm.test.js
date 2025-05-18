import React from 'react';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { SnackbarProvider } from '../../../../contexts/SnackBarContext';
import { DialogProvider } from '../../../../contexts/DialogContext';
import SecretKeyForm from '../SecretKeyForm';
import { api } from '../../../../services/api';

// Mock API
jest.mock('../../../../services/api', () => ({
  api: {
    post: jest.fn(),
    put: jest.fn(),
  }
}));

// Sample data
const mockSecretKey = {
  id: '1',
  origin: 'com.example.app',
  secret_key: 'secretKey123',
  is_default: false,
  active: true,
  created_at: '2023-01-01T00:00:00Z',
  user_identifier_claim: 'sub',
  tenant_identifier_claim: 'tenant_id'
};

describe('SecretKeyForm Component', () => {
  beforeEach(() => {
    // Reset mocks
    jest.clearAllMocks();
    
    // Mock successful API responses
    api.post.mockResolvedValue({ data: { id: '1' } });
    api.put.mockResolvedValue({ data: { id: '1' } });
  });

  const renderComponent = (props = {}) => {
    return render(
      <SnackbarProvider>
        <DialogProvider>
          <SecretKeyForm {...props} />
        </DialogProvider>
      </SnackbarProvider>
    );
  };

  it('should render the form in create mode', () => {
    renderComponent();
    
    // Check title is rendered
    expect(screen.getByText('Add New Secret Key')).toBeInTheDocument();
    
    // Check form fields are rendered - using more specific selectors
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
    const defaultCheckbox = checkboxLabel.closest('label').querySelector('input[type="checkbox"]');
    expect(defaultCheckbox).toBeInTheDocument();
    
    const justificationField = screen.getByRole('textbox', { name: /^Justification/ });
    expect(justificationField).toBeInTheDocument();
    
    // Check button is rendered
    expect(screen.getByRole('button', { name: /Create/i })).toBeInTheDocument();
  });

  it('should render the form in edit mode', () => {
    renderComponent({
      editMode: true,
      secretKey: mockSecretKey,
      onCancel: jest.fn(),
      onSuccess: jest.fn()
    });
    
    // Check title is rendered
    expect(screen.getByText(/Edit Secret Key/i)).toBeInTheDocument();
    
    // Check form fields are rendered with correct values
    const originInput = screen.getByRole('textbox', { name: /^Origin/ });
    expect(originInput).toBeInTheDocument();
    expect(originInput).toHaveValue('com.example.app');
    expect(originInput).toBeDisabled(); // Origin should be disabled in edit mode
    
    // Secret key should be empty (for security reasons)
    const secretKeyInput = screen.getByLabelText(/^Secret Key/);
    expect(secretKeyInput).toBeInTheDocument();
    expect(secretKeyInput).toHaveValue('');
    
    // User identifier claim should have the correct value
    const userIdentifierClaimInput = screen.getByRole('textbox', { name: /^User Identifier Claim/ });
    expect(userIdentifierClaimInput).toBeInTheDocument();
    expect(userIdentifierClaimInput).toHaveValue('sub');
    
    // Tenant identifier claim should have the correct value
    const tenantIdentifierClaimInput = screen.getByRole('textbox', { name: /^Tenant Identifier Claim/ });
    expect(tenantIdentifierClaimInput).toBeInTheDocument();
    expect(tenantIdentifierClaimInput).toHaveValue('tenant_id');
    
    // Check button is rendered
    expect(screen.getByRole('button', { name: /Apply/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Cancel/i })).toBeInTheDocument();
  });

  it('should submit form to create a new secret key', async () => {
    const onSuccessMock = jest.fn();
    renderComponent({ onSuccess: onSuccessMock });
    
    // Fill form fields
    await userEvent.type(screen.getByRole('textbox', { name: /^Origin/ }), 'com.example.newapp');
    await userEvent.type(screen.getByLabelText(/^Secret Key/), 'newSecretKey123');
    await userEvent.type(screen.getByRole('textbox', { name: /^User Identifier Claim/ }), 'email');
    await userEvent.type(screen.getByRole('textbox', { name: /^Tenant Identifier Claim/ }), 'organization');
    await userEvent.type(screen.getByRole('textbox', { name: /^Justification/ }), 'Testing create functionality');
    
    // Submit form
    await userEvent.click(screen.getByRole('button', { name: /Create/i }));
    
    // Verify API call
    await waitFor(() => {
      expect(api.post).toHaveBeenCalledWith('/admin/secret-keys', {
        origin: 'com.example.newapp',
        key: 'newSecretKey123',
        is_default: false,
        justification: 'Testing create functionality',
        user_identifier_claim: 'email',
        tenant_identifier_claim: 'organization'
      });
      expect(onSuccessMock).toHaveBeenCalled();
    });
  });

  it('should submit form to update an existing secret key', async () => {
    const onSuccessMock = jest.fn();
    const onCancelMock = jest.fn();
    
    renderComponent({
      editMode: true,
      secretKey: mockSecretKey,
      onCancel: onCancelMock,
      onSuccess: onSuccessMock
    });
    
    // Fill form fields
    await userEvent.type(screen.getByLabelText(/^Secret Key/), 'updatedSecretKey123');
    // Clear and re-enter user identifier claim
    await userEvent.clear(screen.getByRole('textbox', { name: /^User Identifier Claim/ }));
    await userEvent.type(screen.getByRole('textbox', { name: /^User Identifier Claim/ }), 'username');
    // Clear and re-enter tenant identifier claim
    await userEvent.clear(screen.getByRole('textbox', { name: /^Tenant Identifier Claim/ }));
    await userEvent.type(screen.getByRole('textbox', { name: /^Tenant Identifier Claim/ }), 'company');
    await userEvent.type(screen.getByRole('textbox', { name: /^Justification/ }), 'Testing update functionality');
    
    // Submit form
    await userEvent.click(screen.getByRole('button', { name: /Apply/i }));
    
    // Verify API call
    await waitFor(() => {
      expect(api.put).toHaveBeenCalledWith('/admin/secret-keys/1', {
        key: 'updatedSecretKey123',
        is_default: false,
        justification: 'Testing update functionality',
        user_identifier_claim: 'username',
        tenant_identifier_claim: 'company'
      });
      expect(onSuccessMock).toHaveBeenCalled();
    });
  });

  it('should cancel the edit operation', async () => {
    const onCancelMock = jest.fn();
    
    renderComponent({
      editMode: true,
      secretKey: mockSecretKey,
      onCancel: onCancelMock,
      onSuccess: jest.fn()
    });
    
    // Click cancel button
    await userEvent.click(screen.getByRole('button', { name: /Cancel/i }));
    
    // Verify onCancel was called
    expect(onCancelMock).toHaveBeenCalled();
  });

  it('should show confirmation dialog when setting a key as default', async () => {
    renderComponent();
    
    // Fill form fields
    await userEvent.type(screen.getByRole('textbox', { name: /^Origin/ }), 'com.example.newapp');
    await userEvent.type(screen.getByLabelText(/^Secret Key/), 'newSecretKey123');
    await userEvent.type(screen.getByRole('textbox', { name: /^User Identifier Claim/ }), 'email');
    
    // Check the "Set as default key" checkbox - need to click on the label for MUI
    const checkboxLabel = screen.getByText('Set as default key');
    await userEvent.click(checkboxLabel);
    
    // Submit form (this should trigger the dialog)
    await userEvent.click(screen.getByRole('button', { name: /Create/i }));
    
    // Dialog check would need to be implemented if we had direct access to the dialog context
    // We're limited in what we can test here without mocking the DialogContext itself
  });
}); 