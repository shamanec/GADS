import { useState, useEffect } from 'react';
import { 
  Box, 
  Table, 
  TableBody, 
  TableCell, 
  TableContainer, 
  TableHead, 
  TableRow, 
  Paper, 
  Button,
  CircularProgress,
  TablePagination,
  Chip
} from '@mui/material';
import { FiSearch } from 'react-icons/fi';
import HistoryIcon from '@mui/icons-material/History';
import { api } from '../../../services/api';
import { useSnackbar } from '../../../contexts/SnackBarContext';
import { useDialog } from '../../../contexts/DialogContext';

export default function SecretKeyList({ secretKeys, loading, onEdit, onReload, searchTerm, onSearchChange, onAddClick, onHistoryClick }) {
  const [filteredKeys, setFilteredKeys] = useState([]);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(10);
  const { showSnackbar } = useSnackbar();
  const { showDialog } = useDialog();

  useEffect(() => {
    if (Array.isArray(secretKeys)) {
      const filtered = secretKeys.filter(key => 
        !searchTerm || key.origin.toLowerCase().includes(searchTerm.toLowerCase())
      );
      setFilteredKeys(filtered);
    } else {
      setFilteredKeys([]);
    }
  }, [secretKeys, searchTerm]);

  const handleDisable = (secretKey) => {
    if (secretKey.is_default) {
      showDialog('disableDefaultError', {
        title: 'Cannot Disable Default Key',
        content: 'The default key cannot be disabled. Please set another key as default first if you want to disable this one.',
        isCloseable: true,
        actions: [
          {
            label: 'OK',
            onClick: () => {}
          }
        ]
      });
      return;
    }

    showDialog('confirmDisable', {
      title: 'Confirm Disable',
      content: `Are you sure you want to disable the secret key for origin "${secretKey.origin}"? This may affect active JWT tokens issued for this origin.`,
      isCloseable: true,
      actions: [
        {
          label: 'Cancel',
          onClick: () => {}
        },
        {
          label: 'Disable',
          onClick: () => disableSecretKey(secretKey.id)
        }
      ]
    });
  };

  const disableSecretKey = async (id) => {
    try {
      await api.delete(`/admin/secret-keys/${id}`);
      showSnackbar({
        message: 'Secret key disabled successfully',
        severity: 'success',
        duration: 3000,
      });
      onReload();
    } catch (error) {
      const message = error.response?.data?.error || 'Failed to disable secret key';
      showSnackbar({
        message,
        severity: 'error',
        duration: 3000,
      });
    }
  };

  const handleChangePage = (event, newPage) => {
    setPage(newPage);
  };

  const handleChangeRowsPerPage = (event) => {
    setRowsPerPage(parseInt(event.target.value, 10));
    setPage(0);
  };

  const handleSearchChange = () => {
    const searchInput = document.getElementById('search-input-secret-keys');
    if (searchInput && onSearchChange) {
      onSearchChange(searchInput.value);
    }
  };

  // Calculate pagination
  const paginatedKeys = filteredKeys.slice(
    page * rowsPerPage,
    page * rowsPerPage + rowsPerPage
  );

  return (
    <Box className="secret-key-list">
      <div style={{ width: '100%', display: 'flex', justifyContent: 'space-between', marginBottom: '10px' }}>
        <SearchBox
          keyUpFilterFunc={handleSearchChange}
        />
        <div style={{ display: 'flex', gap: '10px' }}>
          <Button 
            variant="outlined" 
            startIcon={<HistoryIcon />} 
            onClick={onHistoryClick}
            style={{ height: 'fit-content', paddingTop: '8px', paddingBottom: '8px' }}
          >
            View History
          </Button>
          <Button 
            variant="contained" 
            onClick={onAddClick}
            style={{ height: 'fit-content', paddingTop: '8px', paddingBottom: '8px' }}
          >
            Add Secret Key
          </Button>
        </div>
      </div>

      {loading ? (
        <Box sx={{ display: 'flex', justifyContent: 'center', p: 3 }}>
          <CircularProgress />
        </Box>
      ) : filteredKeys.length === 0 ? (
        <Box className="empty-state">
          <p>No secret keys found. Add your first secret key using the button above.</p>
        </Box>
      ) : (
        <TableContainer component={Paper}>
          <Table className="secret-key-table">
            <TableHead>
              <TableRow>
                <TableCell className="table-header">Origin</TableCell>
                <TableCell className="table-header">User Identifier Claim</TableCell>
                <TableCell className="table-header">Tenant Identifier Claim</TableCell>
                <TableCell className="table-header">Status</TableCell>
                <TableCell className="table-header">Created At</TableCell>
                <TableCell className="table-header">Updated At</TableCell>
                <TableCell className="table-header">Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {paginatedKeys.map((secretKey) => (
                <TableRow key={secretKey.id}>
                  <TableCell>{secretKey.origin}</TableCell>
                  <TableCell>{secretKey.user_identifier_claim || '-'}</TableCell>
                  <TableCell>{secretKey.tenant_identifier_claim || '-'}</TableCell>
                  <TableCell>
                    {secretKey.is_default ? (
                      <Chip label="Default" color="primary" size="small" />
                    ) : (
                      <Chip label="Standard" color="default" size="small" />
                    )}
                  </TableCell>
                  <TableCell>
                    {new Date(secretKey.created_at).toLocaleString()}
                  </TableCell>
                  <TableCell>
                    {new Date(secretKey.updated_at).toLocaleString()}
                  </TableCell>
                  <TableCell>
                    <Button 
                      variant="contained"
                      style={{ marginRight: '10px' }}
                      onClick={() => onEdit(secretKey)}
                      disabled={secretKey.disabled === true}
                    >
                      Edit
                    </Button>
                    <Button 
                      variant="contained" 
                      color="error" 
                      onClick={() => handleDisable(secretKey)}
                      disabled={secretKey.disabled === true}
                    >
                      Disable
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          <TablePagination
            rowsPerPageOptions={[5, 10, 25]}
            component="div"
            count={filteredKeys.length}
            rowsPerPage={rowsPerPage}
            page={page}
            onPageChange={handleChangePage}
            onRowsPerPageChange={handleChangeRowsPerPage}
          />
        </TableContainer>
      )}
    </Box>
  );
}

function SearchBox({ keyUpFilterFunc }) {
  return (
    <div id='search-wrapper'>
      <div id='image-wrapper'>
        <FiSearch size={25} />
      </div>
      <input
        type='search'
        id='search-input-secret-keys'
        onInput={() => keyUpFilterFunc()}
        placeholder='Filter by origin'
        className='custom-placeholder'
        autoComplete='off'
      />
    </div>
  );
} 