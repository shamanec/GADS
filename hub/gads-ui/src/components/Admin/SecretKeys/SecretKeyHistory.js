import { useState, useEffect, useCallback } from 'react';
import { 
  Box, 
  Button, 
  Paper, 
  Table, 
  TableBody, 
  TableCell, 
  TableContainer, 
  TableHead, 
  TableRow,
  Typography,
  CircularProgress,
  TextField,
  Chip,
  TablePagination,
  Grid,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  Stack
} from '@mui/material';
import ClearIcon from '@mui/icons-material/Clear';
import ArrowBackIcon from '@mui/icons-material/ArrowBack';
import FilterListIcon from '@mui/icons-material/FilterList';
import { FiSearch } from 'react-icons/fi';
import { getSecretKeyHistory } from '../../../services/secretKeyService';
import { useSnackbar } from '../../../contexts/SnackBarContext';

export default function SecretKeyHistory({ onBack }) {
  const [historyData, setHistoryData] = useState([]);
  const [loading, setLoading] = useState(true);
  const [localFilter, setLocalFilter] = useState('');
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(10);
  const [total, setTotal] = useState(0);
  const [showAdvancedFilters, setShowAdvancedFilters] = useState(false);
  const [filterOrigin, setFilterOrigin] = useState('');
  const [filterAction, setFilterAction] = useState('');
  const [filterUser, setFilterUser] = useState('');
  const [filterFromDate, setFilterFromDate] = useState('');
  const [filterToDate, setFilterToDate] = useState('');
  const [uniqueOrigins, setUniqueOrigins] = useState([]);
  const [uniqueUsers, setUniqueUsers] = useState([]);
  
  const { showSnackbar } = useSnackbar();

  const fetchHistory = useCallback(async () => {
    setLoading(true);
    try {
      // Prepare filters for the API
      const apiFilters = {};
      if (filterOrigin) apiFilters.origin = filterOrigin;
      if (filterAction) apiFilters.action = filterAction;
      if (filterUser) apiFilters.username = filterUser;
      if (filterFromDate) apiFilters.fromDate = new Date(filterFromDate).toISOString();
      if (filterToDate) apiFilters.toDate = new Date(filterToDate).toISOString();

      const response = await getSecretKeyHistory(page + 1, rowsPerPage, apiFilters);
      setHistoryData(response.items || []);
      setTotal(response.total || 0);
      
      // Extract unique values for dropdowns
      if (response.items && response.items.length > 0) {
        const origins = [...new Set(response.items.map(item => item.origin))];
        const users = [...new Set(response.items.map(item => item.user))];
        
        setUniqueOrigins(origins);
        setUniqueUsers(users);
      }
    } catch (error) {
      const message = error.response?.data?.error || 'Failed to fetch history data';
      showSnackbar({
        message,
        severity: 'error',
        duration: 3000,
      });
    } finally {
      setLoading(false);
    }
  }, [page, rowsPerPage, filterOrigin, filterAction, filterUser, filterFromDate, filterToDate, showSnackbar]);

  useEffect(() => {
    fetchHistory();
  }, [fetchHistory]);

  const handleChangePage = (event, newPage) => {
    setPage(newPage);
  };

  const handleChangeRowsPerPage = (event) => {
    setRowsPerPage(parseInt(event.target.value, 10));
    setPage(0); // Reset to first page
  };

  const handleClearFilters = () => {
    setFilterOrigin('');
    setFilterAction('');
    setFilterUser('');
    setFilterFromDate('');
    setFilterToDate('');
    setLocalFilter('');
    setPage(0);
  };

  const filteredHistory = historyData.filter(item => 
    !localFilter || 
    item.origin.toLowerCase().includes(localFilter.toLowerCase()) ||
    item.action.toLowerCase().includes(localFilter.toLowerCase()) ||
    (item.justification && item.justification.toLowerCase().includes(localFilter.toLowerCase())) ||
    (item.user && item.user.toLowerCase().includes(localFilter.toLowerCase()))
  );

  const getActionColor = (action) => {
    switch (action.toLowerCase()) {
      case 'create':
        return 'success';
      case 'update':
        return 'primary';
      case 'disable':
        return 'error';
      default:
        return 'default';
    }
  };

  return (
    <Box className="history-container">
      <Box sx={{ display: 'flex', alignItems: 'center', mb: 3 }}>
        <Button 
          startIcon={<ArrowBackIcon />} 
          onClick={onBack}
          variant="outlined"
        >
          Back to Secret Keys
        </Button>
        <Typography variant="h6" sx={{ ml: 2 }}>
          Secret Keys Audit History
        </Typography>
      </Box>

      <div style={{ width: '100%', display: 'flex', justifyContent: 'space-between', marginBottom: '20px' }}>
        <div id='search-wrapper'>
          <div id='image-wrapper'>
            <FiSearch size={25} />
          </div>
          <input
            id='search-input-history'
            type='search'
            placeholder='Filter results'
            className='custom-placeholder'
            value={localFilter}
            onChange={(e) => setLocalFilter(e.target.value)}
          />
        </div>
        <Stack direction="row" spacing={2}>
          <Button 
            startIcon={<FilterListIcon />}
            onClick={() => setShowAdvancedFilters(!showAdvancedFilters)}
            variant={showAdvancedFilters ? "contained" : "outlined"}
          >
            Advanced Filters
          </Button>
          {(filterOrigin || filterAction || filterUser || filterFromDate || filterToDate) && (
            <Button
              startIcon={<ClearIcon />}
              onClick={handleClearFilters}
              variant="outlined"
            >
              Clear Filters
            </Button>
          )}
        </Stack>
      </div>

      {showAdvancedFilters && (
        <Paper sx={{ p: 2, mb: 3, backgroundColor: '#f4e6cd' }} elevation={0}>
          <Grid container spacing={2}>
            <Grid item xs={12} sm={6} md={3}>
              <FormControl fullWidth size="small">
                <InputLabel id="origin-select-label">Origin</InputLabel>
                <Select
                  labelId="origin-select-label"
                  value={filterOrigin}
                  label="Origin"
                  onChange={(e) => setFilterOrigin(e.target.value)}
                >
                  <MenuItem value="">Any</MenuItem>
                  {uniqueOrigins.map((origin) => (
                    <MenuItem key={origin} value={origin}>{origin}</MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
            <Grid item xs={12} sm={6} md={3}>
              <FormControl fullWidth size="small">
                <InputLabel id="action-select-label">Action</InputLabel>
                <Select
                  labelId="action-select-label"
                  value={filterAction}
                  label="Action"
                  onChange={(e) => setFilterAction(e.target.value)}
                >
                  <MenuItem value="">Any</MenuItem>
                  <MenuItem value="create">Create</MenuItem>
                  <MenuItem value="update">Update</MenuItem>
                  <MenuItem value="disable">Disable</MenuItem>
                </Select>
              </FormControl>
            </Grid>
            <Grid item xs={12} sm={6} md={3}>
              <FormControl fullWidth size="small">
                <InputLabel id="user-select-label">User</InputLabel>
                <Select
                  labelId="user-select-label"
                  value={filterUser}
                  label="User"
                  onChange={(e) => setFilterUser(e.target.value)}
                >
                  <MenuItem value="">Any</MenuItem>
                  {uniqueUsers.map((user) => (
                    <MenuItem key={user} value={user}>{user}</MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Grid>
            <Grid item xs={12} sm={6} md={3}>
              <TextField
                label="From Date"
                type="date"
                value={filterFromDate}
                onChange={(e) => setFilterFromDate(e.target.value)}
                InputLabelProps={{ shrink: true }}
                fullWidth
                size="small"
              />
            </Grid>
            <Grid item xs={12} sm={6} md={3}>
              <TextField
                label="To Date"
                type="date"
                value={filterToDate}
                onChange={(e) => setFilterToDate(e.target.value)}
                InputLabelProps={{ shrink: true }}
                fullWidth
                size="small"
              />
            </Grid>
            <Grid item xs={12} sm={6} md={3}>
              <Button 
                variant="contained" 
                onClick={fetchHistory}
                fullWidth
              >
                Apply Filters
              </Button>
            </Grid>
          </Grid>
        </Paper>
      )}

      {loading ? (
        <Box sx={{ display: 'flex', justifyContent: 'center', p: 3 }}>
          <CircularProgress />
        </Box>
      ) : historyData.length === 0 ? (
        <Box className="empty-state">
          <p>No history data found matching your filters.</p>
        </Box>
      ) : (
        <TableContainer component={Paper}>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell className="table-header">Date</TableCell>
                <TableCell className="table-header">Origin</TableCell>
                <TableCell className="table-header">Action</TableCell>
                <TableCell className="table-header">User</TableCell>
                <TableCell className="table-header">Justification</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {filteredHistory.map((item, index) => (
                <TableRow key={index}>
                  <TableCell>
                    {new Date(item.timestamp).toLocaleString()}
                  </TableCell>
                  <TableCell>{item.origin}</TableCell>
                  <TableCell>
                    <Chip 
                      label={item.action} 
                      color={getActionColor(item.action)}
                      size="small" 
                    />
                  </TableCell>
                  <TableCell>{item.user || 'System'}</TableCell>
                  <TableCell>{item.justification || '-'}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          <TablePagination
            rowsPerPageOptions={[5, 10, 25]}
            component="div"
            count={total}
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