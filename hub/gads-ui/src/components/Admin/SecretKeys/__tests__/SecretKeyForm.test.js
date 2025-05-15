import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
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
  created_at: '2023-01-01T00:00:00Z'
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
    
    // Check form fields are rendered
    expect(screen.getByLabelText(/Origin/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/Secret Key/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/Set as default key/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/Justification/i)).toBeInTheDocument();
    
    // Check button is rendered
    expect(screen.getByText('Create Secret Key')).toBeInTheDocument();
  });

  it('should render the form in edit mode', () => {
    renderComponent({
      editMode: true,
      secretKey: mockSecretKey,
      onCancel: jest.fn(),
      onSuccess: jest.fn()
    });
    
    // Check title is rendered
    expect(screen.getByText('Edit Secret Key')).toBeInTheDocument();
    
    // Check form fields are rendered with correct values
    const originInput = screen.getByLabelText(/Origin/i);
    expect(originInput).toBeInTheDocument();
    expect(originInput).toHaveValue('com.example.app');
    expect(originInput).toBeDisabled(); // Origin should be disabled in edit mode
    
    // Secret key should be empty (for security reasons)
    const secretKeyInput = screen.getByLabelText(/Secret Key/i);
    expect(secretKeyInput).toBeInTheDocument();
    expect(secretKeyInput).toHaveValue('');
    
    // Check button is rendered
    expect(screen.getByText('Update Secret Key')).toBeInTheDocument();
    expect(screen.getByText('Cancel')).toBeInTheDocument();
  });

  it('should submit form to create a new secret key', async () => {
    const onSuccessMock = jest.fn();
    renderComponent({ onSuccess: onSuccessMock });
    
    // Fill form fields
    await userEvent.type(screen.getByLabelText(/Origin/i), 'com.example.newapp');
    await userEvent.type(screen.getByLabelText(/Secret Key/i), 'newSecretKey123');
    await userEvent.type(screen.getByLabelText(/Justification/i), 'Testing create functionality');
    
    // Submit form
    await userEvent.click(screen.getByText('Create Secret Key'));
    
    // Verify API call
    await waitFor(() => {
      expect(api.post).toHaveBeenCalledWith('/admin/secret-keys', {
        origin: 'com.example.newapp',
        secret_key: 'newSecretKey123',
        is_default: false,
        justification: 'Testing create functionality'
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
    await userEvent.type(screen.getByLabelText(/Secret Key/i), 'updatedSecretKey123');
    await userEvent.type(screen.getByLabelText(/Justification/i), 'Testing update functionality');
    
    // Submit form
    await userEvent.click(screen.getByText('Update Secret Key'));
    
    // Verify API call
    await waitFor(() => {
      expect(api.put).toHaveBeenCalledWith('/admin/secret-keys/1', {
        secret_key: 'updatedSecretKey123',
        is_default: false,
        justification: 'Testing update functionality'
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
    await userEvent.click(screen.getByText('Cancel'));
    
    // Verify onCancel was called
    expect(onCancelMock).toHaveBeenCalled();
  });

  it('should show confirmation dialog when setting a key as default', async () => {
    renderComponent();
    
    // Fill form fields
    await userEvent.type(screen.getByLabelText(/Origin/i), 'com.example.newapp');
    await userEvent.type(screen.getByLabelText(/Secret Key/i), 'newSecretKey123');
    
    // Check the "Set as default key" checkbox
    await userEvent.click(screen.getByLabelText(/Set as default key/i));
    
    // Submit form (this should trigger the dialog)
    await userEvent.click(screen.getByText('Create Secret Key'));
    
    // Dialog check would need to be implemented if we had direct access to the dialog context
    // We're limited in what we can test here without mocking the DialogContext itself
  });
}); 