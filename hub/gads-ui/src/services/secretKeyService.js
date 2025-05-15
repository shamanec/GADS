import { api } from './api';

// Get all secret keys
export const getAllSecretKeys = async () => {
  try {
    const response = await api.get('/admin/secret-keys');
    return response.data;
  } catch (error) {
    throw error;
  }
};

// Get a specific secret key by ID
export const getSecretKeyById = async (id) => {
  try {
    const response = await api.get(`/admin/secret-keys/${id}`);
    return response.data;
  } catch (error) {
    throw error;
  }
};

// Create a new secret key
export const createSecretKey = async (secretKeyData) => {
  try {
    const response = await api.post('/admin/secret-keys', secretKeyData);
    return response.data;
  } catch (error) {
    throw error;
  }
};

// Update an existing secret key
export const updateSecretKey = async (id, secretKeyData) => {
  try {
    const response = await api.put(`/admin/secret-keys/${id}`, secretKeyData);
    return response.data;
  } catch (error) {
    throw error;
  }
};

// Disable a secret key
export const disableSecretKey = async (id, justification) => {
  try {
    const response = await api.delete(`/admin/secret-keys/${id}`, { data: { justification } });
    return response.data;
  } catch (error) {
    throw error;
  }
};

// Get secret key audit history
export const getSecretKeyHistory = async (page = 1, limit = 10, filters = {}) => {
  try {
    // Build URL with query parameters
    let url = `/admin/secret-keys/history?page=${page}&limit=${limit}`;
    
    // Add optional filters
    if (filters.origin) {
      url += `&origin=${encodeURIComponent(filters.origin)}`;
    }
    if (filters.action) {
      url += `&action=${encodeURIComponent(filters.action)}`;
    }
    if (filters.user) {
      url += `&user_id=${encodeURIComponent(filters.user)}`;
    }
    if (filters.fromDate) {
      url += `&from_date=${encodeURIComponent(filters.fromDate)}`;
    }
    if (filters.toDate) {
      url += `&to_date=${encodeURIComponent(filters.toDate)}`;
    }
    
    const response = await api.get(url);
    return response.data;
  } catch (error) {
    throw error;
  }
};

// Get a specific audit log by ID
export const getSecretKeyHistoryById = async (id) => {
  try {
    const response = await api.get(`/admin/secret-keys/history/${id}`);
    return response.data;
  } catch (error) {
    throw error;
  }
}; 