import React from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import KeyGenerator from '../KeyGenerator';

// Mock crypto API
const mockRandomValues = jest.fn();
Object.defineProperty(window, 'crypto', {
  value: {
    getRandomValues: mockRandomValues
  }
});

// Mock base64 encoding
global.btoa = jest.fn().mockReturnValue('mockBase64String');

describe('KeyGenerator Component', () => {
  beforeEach(() => {
    // Reset mocks
    jest.clearAllMocks();
    
    // Configure mockRandomValues to fill the array with pseudo-random data
    mockRandomValues.mockImplementation((array) => {
      for (let i = 0; i < array.length; i++) {
        array[i] = Math.floor(Math.random() * 256);
      }
      return array;
    });
  });

  it('should render the generate button', () => {
    render(<KeyGenerator onGenerated={jest.fn()} />);
    
    const button = screen.getByText('Generate Secure Key');
    expect(button).toBeInTheDocument();
  });

  it('should generate a key when button is clicked', async () => {
    const onGeneratedMock = jest.fn();
    render(<KeyGenerator onGenerated={onGeneratedMock} />);
    
    const button = screen.getByText('Generate Secure Key');
    await userEvent.click(button);
    
    // Verify crypto.getRandomValues was called with a 32-byte array
    expect(mockRandomValues).toHaveBeenCalledTimes(1);
    expect(mockRandomValues.mock.calls[0][0].length).toBe(32);
    expect(mockRandomValues.mock.calls[0][0] instanceof Uint8Array).toBe(true);
    
    // Verify btoa was called for base64 encoding
    expect(global.btoa).toHaveBeenCalledTimes(1);
    
    // Verify callback was called with the generated key
    expect(onGeneratedMock).toHaveBeenCalledTimes(1);
    expect(onGeneratedMock).toHaveBeenCalledWith('mockBase64String');
  });

  it('should work even if onGenerated is not provided', async () => {
    // Render without onGenerated prop
    render(<KeyGenerator />);
    
    const button = screen.getByText('Generate Secure Key');
    await userEvent.click(button);
    
    // Should not throw errors
    expect(mockRandomValues).toHaveBeenCalledTimes(1);
    expect(global.btoa).toHaveBeenCalledTimes(1);
  });
}); 