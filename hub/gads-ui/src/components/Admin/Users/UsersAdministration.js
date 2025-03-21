import {
    Box,
    Button,
    FormControl,
    Grid,
    MenuItem,
    TextField, Tooltip,
    Select,
    OutlinedInput,
    InputLabel,
    ListItemText
} from '@mui/material'
import Stack from '@mui/material/Stack'
import { api } from '../../../services/api'
import { useContext, useEffect, useState } from 'react'
import { Auth } from '../../../contexts/Auth'
import './UsersAdministration.css'
import CircularProgress from '@mui/material/CircularProgress'
import CheckIcon from '@mui/icons-material/Check'
import CloseIcon from '@mui/icons-material/Close'
import { useDialog } from '../../../contexts/DialogContext'
import Checkbox from '@mui/material/Checkbox';
import { useSnackbar } from '../../../contexts/SnackBarContext';

const MenuProps = {
    PaperProps: {
        style: {
            maxHeight: 48 * 4.5 + 8,
            width: 250,
        },
    },
};

export default function UsersAdministration() {
    const [userData, setUserData] = useState([])
    const [workspaces, setWorkspaces] = useState([])
    const { showSnackbar } = useSnackbar();

    function handleGetUserData() {
        let url = `/admin/users`
        api.get(url)
            .then(response => {
                setUserData(response.data)
            })
            .catch(e => {
                const message = e.response?.data?.error || 'Failed to get users'
                showSnackbar({
                    message: message,
                    severity: 'error',
                    duration: 3000,
                });
            })

    }

    function fetchWorkspaces() {
        api.get('/admin/workspaces?page=1&limit=100')
            .then(response => {
                setWorkspaces(response.data.workspaces)
            })
            .catch(e => {
                const message = e.response?.data?.error || 'Failed to get workspaces'
                showSnackbar({
                    message: message,
                    severity: 'error',
                    duration: 3000,
                });
            })
    }

    useEffect(() => {

        handleGetUserData()
        fetchWorkspaces()
    }, [])

    return (
        <Stack id='outer-stack' direction='row' spacing={2}>
            <Box id='outer-box'>
                <Grid id='user-grid' container spacing={2}>
                    <Grid item>
                        <NewUser handleGetUserData={handleGetUserData} fetchWorkspaces={fetchWorkspaces} workspaces={workspaces}></NewUser>
                    </Grid>
                    {userData.map((user) => {
                        return (
                            <Grid item>
                                <ExistingUser user={user} handleGetUserData={handleGetUserData} fetchWorkspaces={fetchWorkspaces} workspaces={workspaces}></ExistingUser>
                            </Grid>
                        )
                    })
                    }
                </Grid>
            </Box>
        </Stack>
    )
}

function NewUser({ handleGetUserData, fetchWorkspaces, workspaces }) {
    const [username, setUsername] = useState('')
    const [password, setPassword] = useState('')
    const [role, setRole] = useState('user')
    const [workspaceIds, setWorkspaceIds] = useState([])
    const [loading, setLoading] = useState(false)
    const [addUserStatus, setAddUserStatus] = useState(null)
    const { showSnackbar } = useSnackbar();

    useEffect(() => {
        if (workspaces.length > 0) {
            const defaultWorkspace = workspaces.filter(workspace => workspace.is_default);
            if (defaultWorkspace.length > 0) {
                setWorkspaceIds([defaultWorkspace[0].id]);
            }
        }
    }, [workspaces]);

    function handleAddUser(event) {
        setLoading(true)
        setAddUserStatus(null)
        event.preventDefault()

        const userData = {
            username: username,
            password: password,
            role: role,
            workspace_ids: workspaceIds
        }

        api.post('/admin/user', userData)
            .then(() => {
                setAddUserStatus('success')
                setUsername('')
                setPassword('')
                setRole('user')
                setWorkspaceIds([])
            })
            .catch(e => {
                setAddUserStatus('error')
                const message = e.response?.data?.error || 'Failed to create new user'
                showSnackbar({
                    message: message,
                    severity: 'error',
                    duration: 3000,
                });
            })
            .finally(() => {
                setTimeout(() => {
                    setLoading(false)
                    handleGetUserData()
                    fetchWorkspaces()
                    setTimeout(() => {
                        setAddUserStatus(null)
                    }, 2000)
                }, 1000)
            })
    }

    return (
        <Box className='user-box'>
            <form onSubmit={handleAddUser}>
                <Stack id='user-box-stack' spacing={2}>
                    <Tooltip
                        title='Case-sensitive'
                        arrow
                        placement='top'
                    >
                        <TextField
                            required
                            label='Username'
                            value={username}
                            onChange={(event) => setUsername(event.target.value)}
                            autoComplete='off'
                            size='small'
                        />
                    </Tooltip>
                    <TextField
                        required
                        label='Password'
                        value={password}
                        onChange={(event) => setPassword(event.target.value)}
                        autoComplete='off'
                        size='small'
                    />
                    <FormControl fullWidth required>
                        <TextField
                            value={role}
                            onChange={(e) => setRole(e.target.value)}
                            select
                            label='User role'
                            required
                            size='small'
                        >
                            <MenuItem value='user'>User</MenuItem>
                            <MenuItem value='admin'>Admin</MenuItem>
                        </TextField>
                    </FormControl>
                    <FormControl fullWidth required size="small">
                        <InputLabel id="workspaces-checkbox-label">Workspaces</InputLabel>
                        <Select
                            labelId="workspaces-checkbox-label"
                            id="workspaces-checkbox"
                            multiple
                            value={workspaceIds}
                            onChange={(event) => {
                                const {
                                    target: { value },
                                } = event;
                                setWorkspaceIds(
                                    typeof value === 'string' ? value.split(',') : value,
                                );
                            }}
                            input={<OutlinedInput label="Workspaces" />}
                            renderValue={(selected) => 
                                workspaces
                                    .filter(workspace => selected.includes(workspace.id))
                                    .map(workspace => workspace.name)
                                    .join(', ')
                            }
                            MenuProps={MenuProps}
                        >
                            {workspaces.map((workspace) => (
                                <MenuItem key={workspace.id} value={workspace.id}>
                                    <Checkbox checked={workspaceIds.includes(workspace.id)} />
                                    <ListItemText primary={workspace.name} />
                                </MenuItem>
                            ))}
                        </Select>
                    </FormControl>
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
                        disabled={loading || addUserStatus === 'success' || addUserStatus === 'error'}
                    >
                        {loading ? (
                            <CircularProgress size={25} style={{ color: '#f4e6cd' }} />
                        ) : addUserStatus === 'success' ? (
                            <CheckIcon size={25} style={{ color: '#f4e6cd', stroke: '#f4e6cd', strokeWidth: 2 }} />
                        ) : addUserStatus === 'error' ? (
                            <CloseIcon size={25} style={{ color: 'red', stroke: 'red', strokeWidth: 2 }} />
                        ) : (
                            'Add user'
                        )}
                    </Button>
                </Stack>
            </form>
        </Box>
    )
}

function ExistingUser({ user, handleGetUserData, fetchWorkspaces, workspaces }) {
    const [username, setUsername] = useState(user.username)
    const [password, setPassword] = useState('')
    const [role, setRole] = useState(user.role)
    const [openAlert, setOpenAlert] = useState(false)
    const [updateLoading, setUpdateLoading] = useState(false)
    const [updateUserStatus, setUpdateUserStatus] = useState(null)
    const [workspaceIds, setWorkspaceIds] = useState(user.workspace_ids)
    const { showSnackbar } = useSnackbar();

    function handleUpdateUser(event) {
        setUpdateLoading(true)
        setUpdateUserStatus(null)
        event.preventDefault()

        const updatedUser = {
            username: username,
            password: password,
            role: role,
            workspace_ids: role === 'admin' ? null : workspaceIds
        }

        api.put(`/admin/user`, updatedUser)
            .then(() => {
                setUpdateUserStatus('success')
                setPassword('')
            })
            .catch((e) => {
                setUpdateUserStatus('error')
                const message = e.response?.data?.error || 'Failed to update user'
                showSnackbar({
                    message: message,
                    severity: 'error',
                    duration: 3000,
                });
            })
            .finally(() => {
                setTimeout(() => {
                    setUpdateLoading(false)
                    handleGetUserData()
                    fetchWorkspaces()
                    setTimeout(() => {
                        setUpdateUserStatus(null)
                    }, 2000)
                }, 1000)
            })
    }

    function handleDeleteUser() {
        let url = `/admin/user/${username}`

        api.delete(url)
            .then(() => {
                handleGetUserData()
                fetchWorkspaces()
            })
            .catch((e) => {
                const message = e.response?.data?.error || 'Failed to delete user'
                showSnackbar({
                    message: message,
                    severity: 'error',
                    duration: 3000,
                });
            })
            .finally(() => {
                setOpenAlert(false)
            })
    }

    const { showDialog, hideDialog } = useDialog()
    const showDeleteUserAlert = () => {

        showDialog('deleteUserAlert', {
            title: 'Delete user from DB?',
            content: `Username: ${username}`,
            actions: [
                { label: 'Cancel', onClick: () => hideDialog() },
                { label: 'Confirm', onClick: () => handleDeleteUser() }
            ],
            isCloseable: false
        })
    }

    return (
        <Box className='user-box'>
            <form onSubmit={handleUpdateUser}>
                <Stack id='user-box-stack' spacing={2}
                >
                    <TextField
                        disabled
                        label='Username'
                        value={username}
                        autoComplete='off'
                        size='small'
                        onChange={(event) => setUsername(event.target.value)}
                    />
                    <TextField
                        label='Password'
                        value={password}
                        autoComplete='off'
                        size='small'
                        onChange={(event) => setPassword(event.target.value)}
                    />
                    <FormControl fullWidth required>
                        <TextField
                            value={role}
                            onChange={(e) => setRole(e.target.value)}
                            select
                            label='User role'
                            required
                            size='small'
                            disabled={username === 'admin'}
                        >
                            <MenuItem value='user'>User</MenuItem>
                            <MenuItem value='admin'>Admin</MenuItem>
                        </TextField>
                    </FormControl>
                    {role !== 'admin' && (
                        <FormControl fullWidth required size="small">
                            <InputLabel id="workspaces-checkbox-label">Workspaces</InputLabel>
                            <Select
                                labelId="workspaces-checkbox-label"
                                id="workspaces-checkbox"
                                multiple
                                value={workspaceIds}
                                onChange={(event) => {
                                    const {
                                        target: { value },
                                    } = event;
                                    setWorkspaceIds(
                                        typeof value === 'string' ? value.split(',') : value,
                                    );
                                }}
                                input={<OutlinedInput label="Workspaces" />}
                                renderValue={(selected) => 
                                    workspaces
                                        .filter(workspace => selected.includes(workspace.id))
                                        .map(workspace => workspace.name)
                                        .join(', ')
                                }
                                MenuProps={MenuProps}
                            >
                                {workspaces.map((workspace) => (
                                    <MenuItem key={workspace.id} value={workspace.id}>
                                        <Checkbox checked={workspaceIds.includes(workspace.id)} />
                                        <ListItemText primary={workspace.name} />
                                    </MenuItem>
                                ))}
                            </Select>
                        </FormControl>
                    )}
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
                        disabled={updateLoading || updateUserStatus === 'success' || updateUserStatus === 'error'}
                    >
                        {updateLoading ? (
                            <CircularProgress size={25} style={{ color: '#f4e6cd' }} />
                        ) : updateUserStatus === 'success' ? (
                            <CheckIcon size={25} style={{ color: '#f4e6cd', stroke: '#f4e6cd', strokeWidth: 2 }} />
                        ) : updateUserStatus === 'error' ? (
                            <CloseIcon style={{ color: 'red', stroke: 'red', strokeWidth: 2 }} />
                        ) : (
                            'Update user'
                        )}
                    </Button>
                    <Button
                        disabled={username === 'admin'}
                        variant='contained'
                        style={{
                            backgroundColor: username === 'admin' ? 'gray' : 'orange',
                            color: '#2f3b26',
                            fontWeight: 'bold',
                            height: '40px',
                            boxShadow: 'none'
                        }}
                        onClick={() => showDeleteUserAlert()}
                    >Delete user</Button>
                </Stack>
            </form>
        </Box>
    )
}