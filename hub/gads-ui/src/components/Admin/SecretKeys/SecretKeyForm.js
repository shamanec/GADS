import { useState, useEffect } from 'react';
import { 
  Box, 
  TextField, 
  Button, 
  FormControlLabel, 
  Checkbox, 
  Stack,
  Typography,
  Tooltip,
  CircularProgress
} from '@mui/material';
import { api } from '../../../services/api';
import { useSnackbar } from '../../../contexts/SnackBarContext';
import { useDialog } from '../../../contexts/DialogContext';
import KeyGenerator from './KeyGenerator';

export default function SecretKeyForm({ editMode = false, secretKey = null, onCancel, onSuccess }) {
  const [origin, setOrigin] = useState('');
  const [secret, setSecret] = useState('');
  const [isDefault, setIsDefault] = useState(false);
  const [isDefaultOriginal, setIsDefaultOriginal] = useState(false);
  const [loading, setLoading] = useState(false);
  const [justification, setJustification] = useState('');
  const { showSnackbar } = useSnackbar();
  const { showDialog } = useDialog();

  useEffect(() => {
    if (editMode && secretKey) {
      setOrigin(secretKey.origin || '');
      setSecret(''); // Don't populate for security reasons
      setIsDefault(secretKey.is_default || false);
      setIsDefaultOriginal(secretKey.is_default || false);
    } else {
      // Reset the form when not in edit mode
      setOrigin('');
      setSecret('');
      setIsDefault(false);
      setIsDefaultOriginal(false);
      setJustification('');
    }
  }, [editMode, secretKey]);

  const handleSubmit = async (event) => {
    event.preventDefault();
    setLoading(true);

    try {
      if (editMode) {
        if (isDefault !== isDefaultOriginal) {
          showDefaultChangeConfirmation();
          setLoading(false);
          return;
        }

        await updateSecretKey();
      } else {
        if (isDefault) {
          showDefaultChangeConfirmation();
          setLoading(false);
          return;
        } else {
          await createSecretKey();
        }
      }

      showSnackbar({
        message: `Secret key ${editMode ? 'updated' : 'created'} successfully`,
        severity: 'success',
        duration: 3000,
      });

      if (onSuccess) {
        onSuccess();
      }
    } catch (error) {
      const message = error.response?.data?.error || `Failed to ${editMode ? 'update' : 'create'} secret key`;
      showSnackbar({
        message,
        severity: 'error',
        duration: 3000,
      });
    } finally {
      setLoading(false);
    }
  };

  const createSecretKey = async () => {
    await api.post('/admin/secret-keys', {
      origin,
      key: secret,
      is_default: isDefault,
      justification: justification || undefined
    });
  };

  const updateSecretKey = async () => {
    // Only send fields that were actually modified
    const updateData = {
      is_default: isDefault,
    };
    
    // Add key only if provided
    if (secret.trim() !== '') {
      updateData.key = secret;
    }
    
    // Add justification if provided
    if (justification.trim() !== '') {
      updateData.justification = justification;
    }
    
    await api.put(`/admin/secret-keys/${secretKey.id}`, updateData);
  };

  const showDefaultChangeConfirmation = () => {
    showDialog('defaultKeyConfirmation', {
      title: 'Default Key Change',
      content: 'Setting this key as the default will remove the default status from any other key. New tokens for unknown origins will use this key. Existing tokens will not continue to work unless this key matches the previously default one. Do you want to continue?',
      isCloseable: true,
      actions: [
        {
          label: 'Cancel',
          onClick: () => {}
        },
        {
          label: 'Confirm',
          onClick: async () => {
            setLoading(true);
            try {
              if (editMode) {
                await updateSecretKey();
              } else {
                await createSecretKey();
              }
              
              showSnackbar({
                message: `Secret key ${editMode ? 'updated' : 'created'} successfully`,
                severity: 'success',
                duration: 3000,
              });

              if (onSuccess) {
                onSuccess();
              }
            } catch (error) {
              const message = error.response?.data?.error || `Failed to ${editMode ? 'update' : 'create'} secret key`;
              showSnackbar({
                message,
                severity: 'error',
                duration: 3000,
              });
            } finally {
              setLoading(false);
            }
          }
        }
      ]
    });
  };

  const handleGeneratedKey = (key) => {
    setSecret(key);
  };

  return (
    <>
      <h2>{editMode ? `Edit Secret Key: ${secretKey?.origin}` : 'Add New Secret Key'}</h2>
      
      <form onSubmit={handleSubmit}>
        <Stack spacing={3}>
          <TextField
            label="Origin"
            value={origin}
            onChange={(e) => setOrigin(e.target.value)}
            required
            disabled={editMode}
            fullWidth
            size="small"
            helperText="The origin identifier (e.g., 'com.example.app')"
          />
          
          <Box>
            <TextField
              label="Secret Key"
              value={secret}
              onChange={(e) => setSecret(e.target.value)}
              required={!editMode}
              fullWidth
              size="small"
              type="password"
              helperText={editMode ? "Leave blank to keep the current key" : "Enter a secure key or generate one automatically"}
            />
            <Box sx={{ mt: 1 }}>
              <KeyGenerator onGenerated={handleGeneratedKey} />
            </Box>
          </Box>
          
          <FormControlLabel
            control={
              <Checkbox 
                checked={isDefault} 
                onChange={(e) => setIsDefault(e.target.checked)} 
              />
            }
            label={
              <Tooltip 
                title="If checked, this key will be used for unknown origins. Only one key can be the default." 
                arrow
              >
                <span>Set as default key</span>
              </Tooltip>
            }
          />
          
          <TextField
            label="Justification"
            value={justification}
            onChange={(e) => setJustification(e.target.value)}
            multiline
            rows={2}
            fullWidth
            size="small"
            helperText="Optional: Provide a reason for creating/updating this key (for audit logs)"
          />
          
          <Stack direction='row' spacing={1} justifyContent="flex-end">
            <Button 
              variant="outlined" 
              onClick={onCancel} 
              disabled={loading}
            >
              Cancel
            </Button>
            
            <Button 
              type="submit" 
              variant="contained" 
              color="primary" 
              disabled={loading}
            >
              {loading ? (
                <CircularProgress size={24} />
              ) : (
                editMode ? 'Apply' : 'Create'
              )}
            </Button>
          </Stack>
        </Stack>
      </form>
    </>
  );
} 