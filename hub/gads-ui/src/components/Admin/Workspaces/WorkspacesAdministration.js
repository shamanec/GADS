import { useState, useEffect } from 'react';
import { api } from '../../../services/api';
import { Box, Button, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, TableSortLabel, CircularProgress, Paper, Modal, TextField, Stack, TablePagination } from '@mui/material';
import { useSnackbar } from '../../../contexts/SnackBarContext';
import './WorkspacesAdministration.css';
import { FiSearch } from 'react-icons/fi'
import { useDialog } from '../../../contexts/DialogContext';

export default function WorkspacesAdministration() {
    const [workspaces, setWorkspaces] = useState([]);
    const [loading, setLoading] = useState(false);
    const [openModal, setOpenModal] = useState(false);
    const [currentWorkspace, setCurrentWorkspace] = useState(null);
    const [newWorkspace, setNewWorkspace] = useState({ name: '', description: '' });
    const [page, setPage] = useState(0);
    const [rowsPerPage, setRowsPerPage] = useState(10);
    const [totalCount, setTotalCount] = useState(0);
    const [searchTerm, setSearchTerm] = useState('');
    const { showSnackbar } = useSnackbar();
    const { showDialog, hideDialog } = useDialog();

    useEffect(() => {
        fetchWorkspaces();
    }, [page, rowsPerPage, searchTerm]);

    const fetchWorkspaces = async () => {
        setLoading(true);
        try {
            const response = await api.get(`/admin/workspaces?page=${page + 1}&limit=${rowsPerPage}&search=${searchTerm}`);
            setWorkspaces(response.data.workspaces || []);
            setTotalCount(response.data.total);
        } catch (error) {
            showSnackbar({
                message: error.response?.data?.error || 'Failed to fetch workspaces.',
                severity: 'error',
                duration: 3000,
            });
        } finally {
            setLoading(false);
        }
    };

    const handleChangePage = (event, newPage) => {
        setPage(newPage);
    };

    const handleChangeRowsPerPage = (event) => {
        setRowsPerPage(parseInt(event.target.value, 10));
        setPage(0); // Reset to first page
    };

    const handleEditWorkspace = (workspace) => {
        setCurrentWorkspace(workspace);
        setOpenModal(true);
    };

    const handleCloseModal = () => {
        setOpenModal(false);
        setCurrentWorkspace(null);
        setNewWorkspace({ name: '', description: '' });
    };

    const handleUpdateWorkspace = async () => {
        try {
            await api.put(`/admin/workspaces`, {
                id: currentWorkspace.id,
                name: currentWorkspace.name,
                description: currentWorkspace.description,
            });
            showSnackbar({
                message: 'Workspace updated successfully!',
                severity: 'success',
                duration: 3000,
            });
            fetchWorkspaces();
            handleCloseModal();
        } catch (error) {
            showSnackbar({
                message: error.response?.data?.error || 'Failed to update workspace.',
                severity: 'error',
                duration: 3000,
            });
        }
    };

    const handleDeleteWorkspace = (workspace) => {
        showDialog('deleteWorkspaceAlert', {
            title: 'Delete workspace from DB?',
            content: `Workspace: ${workspace.name}`,
            actions: [
                { label: 'Cancel', onClick: () => hideDialog() },
                { label: 'Confirm', onClick: () => {
                    handleConfirmDelete(workspace);
                    hideDialog();
                }}
            ],
            isCloseable: false
        });
    };

    const handleConfirmDelete = async (workspace) => {
        try {
            await api.delete(`/admin/workspaces/${workspace.id}`);
            showSnackbar({
                message: 'Workspace deleted successfully!',
                severity: 'success',
                duration: 3000,
            });
            fetchWorkspaces();
        } catch (error) {
            showSnackbar({
                message: error.response?.data?.error || 'Failed to delete workspace.',
                severity: 'error',
                duration: 3000,
            });
        }
    };

    const handleAddWorkspace = () => {
        setOpenModal(true);
    };

    const handleCreateWorkspace = async () => {
        try {
            await api.post('/admin/workspaces', newWorkspace);
            showSnackbar({
                message: 'Workspace created successfully!',
                severity: 'success',
                duration: 3000,
            });
            fetchWorkspaces();
            handleCloseModal();
        } catch (error) {
            showSnackbar({
                message: error.response?.data?.error || 'Failed to create workspace.',
                severity: 'error',
                duration: 3000,
            });
        }
    };

    const handleSearchChange = (event) => {
        var input = document.getElementById('search-input-workspaces')
        setSearchTerm(input.value);
        setPage(0);
    };

    return (
        <Stack id='outer-stack' direction='row' spacing={2}>
            <Box id='outer-box' className='workspace-managment-container'>
                <div style={{ width: '100%', display: 'flex', justifyContent: 'space-between' }}>
                    <WorkspaceSearch
                        keyUpFilterFunc={handleSearchChange}
                    ></WorkspaceSearch>
                    <Button variant='contained' onClick={handleAddWorkspace} style={{ float: 'right', height: 'fit-content', paddingTop: '8px', paddingBottom: '8px' }}>
                        Add Workspace
                    </Button>
                </div>
                {loading ? (
                    <CircularProgress />
                ) : (
                    <TableContainer style={{ marginTop: '10px' }} component={Paper}>
                        <Table>
                            <TableHead>
                                <TableRow>
                                    <TableCell className="table-header">Workspace Name</TableCell>
                                    <TableCell className="table-header">Description</TableCell>
                                    <TableCell className="table-header">Type</TableCell>
                                    <TableCell className="table-header">Actions</TableCell>
                                </TableRow>
                            </TableHead>
                            <TableBody>
                                {workspaces.map((workspace) => (
                                    <TableRow key={workspace.id}>
                                        <TableCell>{workspace.name}</TableCell>
                                        <TableCell>{workspace.description}</TableCell>
                                        <TableCell>{workspace.is_default ? 'Default' : 'Custom'}</TableCell>
                                        <TableCell>
                                            <Button style={{ marginRight: '10px' }} variant='contained' onClick={() => handleEditWorkspace(workspace)}>
                                                Edit
                                            </Button>
                                            {!workspace.is_default && (
                                                <Button variant='contained' color='error' onClick={() => handleDeleteWorkspace(workspace)}>
                                                    Delete
                                                </Button>
                                            )}
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                        <TablePagination
                            rowsPerPageOptions={[5, 10, 25]}
                            component="div"
                            count={totalCount}
                            rowsPerPage={rowsPerPage}
                            page={page}
                            onPageChange={handleChangePage}
                            onRowsPerPageChange={handleChangeRowsPerPage}
                        />
                    </TableContainer>
                )}

                <Modal open={openModal} onClose={handleCloseModal}>
                    <Box sx={{
                        padding: 4,
                        backgroundColor: 'white',
                        borderRadius: 2,
                        boxShadow: 3,
                        maxWidth: 400,
                        margin: 'auto',
                        mt: 5
                    }}>
                        <h2>{currentWorkspace ? 'Edit Workspace' : 'Add Workspace'}</h2>
                        <Stack spacing={2}>
                            <TextField
                                label='Workspace Name'
                                value={currentWorkspace ? currentWorkspace.name : newWorkspace.name}
                                onChange={(e) => {
                                    if (currentWorkspace) {
                                        setCurrentWorkspace({ ...currentWorkspace, name: e.target.value });
                                    } else {
                                        setNewWorkspace({ ...newWorkspace, name: e.target.value });
                                    }
                                }}
                                required
                            />
                            <TextField
                                label='Description'
                                value={currentWorkspace ? currentWorkspace.description : newWorkspace.description}
                                onChange={(e) => {
                                    if (currentWorkspace) {
                                        setCurrentWorkspace({ ...currentWorkspace, description: e.target.value });
                                    } else {
                                        setNewWorkspace({ ...newWorkspace, description: e.target.value });
                                    }
                                }}
                                required
                            />
                            <Stack direction='row' spacing={1}>
                                <Button variant='contained' onClick={currentWorkspace ? handleUpdateWorkspace : handleCreateWorkspace}>
                                    {currentWorkspace ? 'Apply' : 'Create'}
                                </Button>
                                <Button variant='outlined' onClick={handleCloseModal}>
                                    Cancel
                                </Button>
                            </Stack>
                        </Stack>
                    </Box>
                </Modal>
            </Box>
        </Stack>
    );
}

export function WorkspaceSearch({ keyUpFilterFunc }) {
    return (
        <div id='search-wrapper'>
            <div id='image-wrapper'>
                <FiSearch size={25} />
            </div>
            <input
                type='search'
                id='search-input-workspaces'
                onInput={() => keyUpFilterFunc()}
                placeholder='Search workspaces'
                className='custom-placeholder'
                autoComplete='off'
            ></input>
        </div>
    )
}