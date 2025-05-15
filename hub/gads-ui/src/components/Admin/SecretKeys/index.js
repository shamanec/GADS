import { Box, Stack, Modal } from '@mui/material';
import { useState, useEffect, useCallback } from 'react';
import { api } from '../../../services/api';
import { useSnackbar } from '../../../contexts/SnackBarContext';
import SecretKeyList from './SecretKeyList';
import SecretKeyForm from './SecretKeyForm';
import SecretKeyHistory from './SecretKeyHistory';
import './SecretKeys.css';

export default function SecretKeys() {
  const [secretKeys, setSecretKeys] = useState([]);
  const [loading, setLoading] = useState(false);
  const [showHistory, setShowHistory] = useState(false);
  const [openModal, setOpenModal] = useState(false);
  const [editKey, setEditKey] = useState(null);
  const [reloadTrigger, setReloadTrigger] = useState(0);
  const [searchTerm, setSearchTerm] = useState('');
  const { showSnackbar } = useSnackbar();

  const fetchSecretKeys = useCallback(async () => {
    setLoading(true);
    try {
      const response = await api.get('/admin/secret-keys');
      setSecretKeys(Array.isArray(response.data.secret_keys) ? response.data.secret_keys : []);
    } catch (error) {
      const message = error.response?.data?.error || 'Failed to fetch secret keys';
      showSnackbar({
        message,
        severity: 'error',
        duration: 3000,
      });
      setSecretKeys([]);
    } finally {
      setLoading(false);
    }
  }, [showSnackbar]);

  useEffect(() => {
    fetchSecretKeys();
  }, [reloadTrigger, fetchSecretKeys]);

  const handleReload = () => {
    setReloadTrigger(prev => prev + 1);
  };

  const toggleHistory = () => {
    setShowHistory(prev => !prev);
  };

  const handleAddSecretKey = () => {
    setEditKey(null);
    setOpenModal(true);
  };

  const handleEditSecretKey = (secretKey) => {
    setEditKey(secretKey);
    setOpenModal(true);
  };

  const handleCloseModal = () => {
    setOpenModal(false);
    setEditKey(null);
  };

  const handleFormSuccess = () => {
    handleCloseModal();
    handleReload();
  };

  const handleSearchChange = (term) => {
    setSearchTerm(term);
  };

  return (
    <Stack id='outer-stack' direction='row' spacing={2}>
      <Box id='outer-box' className='secret-keys-container'>
        {showHistory ? (
          <SecretKeyHistory onBack={toggleHistory} />
        ) : (
          <>
            <SecretKeyList 
              secretKeys={secretKeys} 
              loading={loading} 
              onEdit={handleEditSecretKey}
              onReload={handleReload}
              searchTerm={searchTerm}
              onSearchChange={handleSearchChange}
              onAddClick={handleAddSecretKey}
              onHistoryClick={toggleHistory}
            />

            <Modal open={openModal} onClose={handleCloseModal}>
              <Box sx={{
                padding: 4,
                backgroundColor: 'white',
                borderRadius: 2,
                boxShadow: 3,
                maxWidth: 600,
                margin: 'auto',
                mt: 5,
                maxHeight: '90vh',
                overflow: 'auto'
              }}>
                <SecretKeyForm 
                  editMode={!!editKey} 
                  secretKey={editKey} 
                  onCancel={handleCloseModal} 
                  onSuccess={handleFormSuccess} 
                />
              </Box>
            </Modal>
          </>
        )}
      </Box>
    </Stack>
  );
} 