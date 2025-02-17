import { useState, useEffect } from 'react';
import { api } from '../../../services/api';
import { Box, Button, FormControl, Grid, MenuItem, Stack, TextField, Tooltip } from '@mui/material';
import CircularProgress from '@mui/material/CircularProgress';
import CheckIcon from '@mui/icons-material/Check';
import CloseIcon from '@mui/icons-material/Close';
import './WorkspacesAdministration.css';
import { useSnackbar } from '../../../contexts/SnackBarContext';

export default function WorkspacesAdministration() {
    const [workspaces, setWorkspaces] = useState([]);
    const [loading, setLoading] = useState(false);

    useEffect(() => {
        fetchWorkspaces();
    }, []);

    const fetchWorkspaces = async () => {
        const response = await api.get('/admin/workspaces');
        setWorkspaces(response.data);
    };

    return (
        <Stack id='outer-stack' direction='row' spacing={2}>
            <Box id='outer-box'>
                <Grid id='workspace-grid' container spacing={2}>
                    <Grid item>
                        <NewWorkspace handleGetWorkspaces={fetchWorkspaces} />
                    </Grid>
                    {workspaces.map((workspace) => (
                        <Grid item key={workspace.id}>
                            <ExistingWorkspace workspace={workspace} handleGetWorkspaces={fetchWorkspaces} />
                        </Grid>
                    ))}
                </Grid>
            </Box>
        </Stack>
    );
}

function NewWorkspace({ handleGetWorkspaces }) {
    const [name, setName] = useState('');
    const [description, setDescription] = useState('');
    const [loading, setLoading] = useState(false);
    const [addWorkspaceStatus, setAddWorkspaceStatus] = useState(null);
    const { showSnackbar } = useSnackbar();

    const handleAddWorkspace = (event) => {
        setLoading(true);
        setAddWorkspaceStatus(null);
        event.preventDefault();

        const workspaceData = { name, description };
        api.post('/admin/workspaces', workspaceData)
        .then(() => {
            setAddWorkspaceStatus('sucess');
            setName('');
            setDescription('');
        })
        .catch(e => {
            setAddWorkspaceStatus('error')
            showSnackbar({
                message: e.response.data.error,
                severity: 'error',
                duration: 3000,
            });
        })
        .finally(() => {
            setLoading(false)
            handleGetWorkspaces()
            setAddWorkspaceStatus(null)
        })
    };

    return (
        <Box className='workspace-box'>
            <form onSubmit={handleAddWorkspace}>
                <Stack id='workspace-box-stack' spacing={2}>
                    <Tooltip title='Case-sensitive' arrow placement='top'>
                        <TextField
                            required
                            label='Workspace Name'
                            value={name}
                            onChange={(e) => setName(e.target.value)}
                            autoComplete='off'
                            size='small'
                        />
                    </Tooltip>
                    <TextField
                        required
                        label='Description'
                        value={description}
                        onChange={(e) => setDescription(e.target.value)}
                        autoComplete='off'
                        size='small'
                    />
                    <Button
                        variant='contained'
                        type='submit'
                        style={{
                            backgroundColor: '#2f3b26',
                            color: '#f4e6cd',
                            fontWeight: 'bold',
                            boxShadow: 'none',
                            height: '40px'
                        }}
                        disabled={loading}
                    >
                        {loading ? (
                            <CircularProgress size={25} style={{ color: '#f4e6cd' }} />
                        ) : (
                            'Add Workspace'
                        )}
                    </Button>
                </Stack>
            </form>
        </Box>
    );
}

function ExistingWorkspace({ workspace, handleGetWorkspaces }) {
    const [name, setName] = useState(workspace.name);
    const [description, setDescription] = useState(workspace.description);
    const [loading, setLoading] = useState(false);
    const [updateStatus, setUpdateStatus] = useState(null);

    const handleUpdateWorkspace = async (event) => {
        setLoading(true);
        event.preventDefault();

        const updatedWorkspace = { id: workspace.id, name, description };
        await api.put('/admin/workspaces', updatedWorkspace);
        handleGetWorkspaces();
        setLoading(false);
    };

    const handleDeleteWorkspace = async () => {
        await api.delete(`/admin/workspaces/${workspace.id}`);
        handleGetWorkspaces();
    };

    return (
        <Box className='workspace-box'>
            <form onSubmit={handleUpdateWorkspace}>
                <Stack id='workspace-box-stack' spacing={2}>
                    <TextField
                        label='Workspace Name'
                        value={name}
                        onChange={(e) => setName(e.target.value)}
                        autoComplete='off'
                        size='small'
                    />
                    <TextField
                        label='Description'
                        value={description}
                        onChange={(e) => setDescription(e.target.value)}
                        autoComplete='off'
                        size='small'
                    />
                    <Button
                        variant='contained'
                        type='submit'
                        style={{
                            backgroundColor: '#2f3b26',
                            color: '#f4e6cd',
                            fontWeight: 'bold',
                            height: '40px',
                            boxShadow: 'none'
                        }}
                        disabled={loading}
                    >
                        {loading ? (
                            <CircularProgress size={25} style={{ color: '#f4e6cd' }} />
                        ) : (
                            'Update Workspace'
                        )}
                    </Button>
                    {!workspace.is_default && (
                        <Button
                            variant='contained'
                            style={{
                                backgroundColor: 'orange',
                                color: '#2f3b26',
                                fontWeight: 'bold',
                                height: '40px',
                                boxShadow: 'none'
                            }}
                            onClick={handleDeleteWorkspace}
                        >
                            Delete Workspace
                        </Button>
                    )}
                </Stack>
            </form>
        </Box>
    );
} 