import {
    Box,
    Button,
    Dialog,
    DialogActions,
    DialogContent, DialogContentText,
    DialogTitle,
    FormControl,
    Grid,
    MenuItem,
    TextField, Tooltip
} from "@mui/material";
import Stack from '@mui/material/Stack';
import { api } from "../../../services/api";
import { useContext, useEffect, useState } from "react";
import { Auth } from "../../../contexts/Auth";
import './UsersAdministration.css'
import CircularProgress from '@mui/material/CircularProgress';
import CheckIcon from '@mui/icons-material/Check';
import CloseIcon from '@mui/icons-material/Close';

export default function UsersAdministration() {
    const [userData, setUserData] = useState([])
    const { logout } = useContext(Auth)

    function handleGetUserData() {
        let url = `/admin/users`
        api.get(url)
            .then(response => {
                setUserData(response.data)
            })
            .catch(error => {
                if (error.response) {
                    if (error.response.status === 401) {
                        logout()
                    }
                }
            })

    }

    useEffect(() => {
        handleGetUserData()
    }, [])

    return (
        <Stack id='outer-stack' direction='row' spacing={2}>
            <Box id='outer-box'>
                <Grid id='user-grid' container spacing={2}>
                    <Grid item>
                        <NewUser handleGetUserData={handleGetUserData}></NewUser>
                    </Grid>
                    {userData.map((user) => {
                        return (
                            <Grid item>
                                <ExistingUser user={user} handleGetUserData={handleGetUserData}></ExistingUser>
                            </Grid>
                        )
                    })
                    }
                </Grid>
            </Box>
        </Stack>
    )
}

function NewUser({ handleGetUserData }) {
    const [username, setUsername] = useState('')
    const [password, setPassword] = useState('')
    const [role, setRole] = useState('user')
    const [loading, setLoading] = useState(false);
    const [addUserStatus, setAddUserStatus] = useState(null)

    function handleAddUser(event) {
        setLoading(true)
        setAddUserStatus(null)
        event.preventDefault()

        let url = `/admin/user`

        const loginData = {
            username: username,
            password: password,
            role: role
        };

        api.post(url, loginData)
            .then(() => {
                setAddUserStatus('success')
                setUsername('')
                setPassword('')
                setRole('user')
            })
            .catch(e => {
                setAddUserStatus('error')
            })
            .finally(() => {
                setTimeout(() => {
                    setLoading(false)
                    handleGetUserData()
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

function ExistingUser({ user, handleGetUserData }) {
    const [username, setUsername] = useState(user.username)
    const [password, setPassword] = useState('')
    const [role, setRole] = useState(user.role)
    const [openAlert, setOpenAlert] = useState(false)
    const [updateLoading, setUpdateLoading] = useState(false);
    const [updateUserStatus, setUpdateUserStatus] = useState(null)

    function handleUpdateUser(event) {
        setUpdateLoading(true)
        setUpdateUserStatus(null)
        event.preventDefault()

        let url = `/admin/user`

        const loginData = {
            username: username,
            password: password,
            role: role
        };

        api.put(url, loginData)
            .then(() => {
                setUpdateUserStatus('success')
                setPassword('')
            })
            .catch(() => {
                setUpdateUserStatus('error')
            })
            .finally(() => {
                setTimeout(() => {
                    setUpdateLoading(false)
                    handleGetUserData()
                    setTimeout(() => {
                        setUpdateUserStatus(null)
                    }, 2000)
                }, 1000)
            })
    }

    function handleDeleteUser() {
        let url = `/admin/user/${username}`

        api.delete(url)
            .then(() =>
                handleGetUserData()
            )
            .catch()
            .finally(() => {
                setOpenAlert(false)
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
                        onClick={() => setOpenAlert(true)}
                    >Delete user</Button>
                    <Dialog
                        open={openAlert}
                        onClose={() => setOpenAlert(false)}
                    >
                        <DialogTitle>
                            Delete user from DB?
                        </DialogTitle>
                        <DialogContent>
                            <DialogContentText>
                                Username: {username}
                            </DialogContentText>
                        </DialogContent>
                        <DialogActions>
                            <Button onClick={() => setOpenAlert(false)}>Cancel</Button>
                            <Button onClick={handleDeleteUser} autoFocus>
                                Confirm
                            </Button>
                        </DialogActions>
                    </Dialog>
                </Stack>
            </form>
        </Box>
    )
}