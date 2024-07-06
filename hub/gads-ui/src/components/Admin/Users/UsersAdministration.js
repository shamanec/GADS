import { Box, Button, FormControl, Grid, MenuItem, TextField, Tooltip } from "@mui/material";
import Stack from '@mui/material/Stack';
import { api } from "../../../services/api";
import { useContext, useEffect, useState } from "react";
import { Auth } from "../../../contexts/Auth";

export default function UsersAdministration() {
    const [userData, setUserData] = useState([])
    const { logout } = useContext(Auth)

    function handleGetUserData() {
        let url = `/admin/users`
        api.get(url)
            .then(response => {
                console.log('got response')
                console.log(response.data)
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
        <Stack direction='row' spacing={2} style={{ width: '100%', marginLeft: '10px', marginTop: '10px' }}>
            <Box
                style={{
                    marginBottom: '10px',
                    height: '80vh',
                    overflowY: 'scroll',
                    border: '2px solid black',
                    borderRadius: '10px',
                    boxShadow: 'inset 0 -10px 10px -10px #000000',
                    scrollbarWidth: 'none',
                    marginRight: '10px',
                    width: '100%'
                }}
            >
                <Grid
                    container
                    spacing={2}
                    margin='10px'
                >
                    <Grid item>
                        <NewUser handleGetUserData={handleGetUserData}>
                        </NewUser>
                    </Grid>
                    {userData.map((user) => {
                        return (
                            <Grid item>
                                <ExistingUser
                                    user={user}
                                    handleGetUserData={handleGetUserData}
                                >
                                </ExistingUser>
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

    function handleAddUser(event) {
        event.preventDefault()

        let url = `/admin/user`

        const loginData = {
            username: username,
            password: password,
            role: role
        };

        api.post(url, loginData)
            .then(response => {
                handleGetUserData()
                setUsername('')
                setPassword('')
                setRole('user')
            })
            .catch(e => {
                console.log(e)
            })
    }

    return (
        <Box
            id='some-box'
            style={{
                border: '1px solid black',
                width: '400px',
                minWidth: '400px',
                maxWidth: '400px',
                height: '350px',
                borderRadius: '5px',
                backgroundColor: '#9ba984'
            }}
        >
            <form onSubmit={handleAddUser}>
                <Stack
                    spacing={2}
                    style={{
                        padding: '10px'
                    }}
                >
                    <TextField
                        required
                        label="Username"
                        value={username}
                        autoComplete="off"
                        size='small'
                        onChange={(event) => setUsername(event.target.value)}
                        helperText='Username should be at least 4 characters'
                    />
                    <TextField
                        required
                        label="Password"
                        value={password}
                        autoComplete="off"
                        size='small'
                        onChange={(event) => setPassword(event.target.value)}
                        helperText='Password should be minimum 6 chars long, contain upper and lower case letter, digit and special char'
                    />
                    <FormControl fullWidth variant="outlined" required>
                        <TextField
                            style={{ width: "100%" }}
                            variant="outlined"
                            value={role}
                            onChange={(e) => setRole(e.target.value)}
                            select
                            label="User role"
                            required
                            size='small'
                        >
                            <MenuItem value='user'>User</MenuItem>
                            <MenuItem value='admin'>Admin</MenuItem>
                        </TextField>
                    </FormControl>
                    <Button
                        variant="contained"
                        type="submit"
                        style={{
                            backgroundColor: '#2f3b26',
                            color: '#f4e6cd'
                        }}
                    >Add user</Button>
                </Stack>
            </form>
        </Box>
    )
}

function ExistingUser({ user, handleGetUserData }) {
    const [username, setUsername] = useState(user.username)
    const [password, setPassword] = useState('')
    const [role, setRole] = useState(user.role)

    function handleUpdateUser(event) {
        event.preventDefault()

        let url = `/admin/user`

        const loginData = {
            username: username,
            password: password,
            role: role
        };

        api.put(url, loginData)
            .then(response => {
                handleGetUserData()
                setPassword('')
            })
            .catch(e => {
                console.log(e)
            })
    }

    function handleDeleteUser() {
        let url = `/admin/user/${username}`

        api.delete(url)
            .then(() =>
                handleGetUserData()
            )
            .catch()
    }

    return (
        <Box
            id='some-box'
            style={{
                border: '1px solid black',
                width: '400px',
                minWidth: '400px',
                maxWidth: '400px',
                height: '350px',
                borderRadius: '5px',
                backgroundColor: '#9ba984'
            }}
        >
            <form onSubmit={handleUpdateUser}>
                <Stack
                    spacing={2}
                    style={{
                        padding: '10px'
                    }}
                >
                    <TextField
                        disabled
                        label="Username"
                        value={username}
                        autoComplete="off"
                        size='small'
                        onChange={(event) => setUsername(event.target.value)}
                        helperText='Username should be at least 4 characters'
                    />
                    <TextField
                        label="Password"
                        value={password}
                        autoComplete="off"
                        size='small'
                        onChange={(event) => setPassword(event.target.value)}
                        helperText='Password should be minimum 6 chars long, contain upper and lower case letter, digit and special char'
                    />
                    <FormControl fullWidth variant="outlined" required>
                        <TextField
                            style={{ width: "100%" }}
                            variant="outlined"
                            value={role}
                            onChange={(e) => setRole(e.target.value)}
                            select
                            label="User role"
                            required
                            size='small'
                            disabled={username === "admin"}
                        >
                            <MenuItem value='user'>User</MenuItem>
                            <MenuItem value='admin'>Admin</MenuItem>
                        </TextField>
                    </FormControl>
                    <Button
                        variant="contained"
                        type="submit"
                        style={{
                            backgroundColor: '#2f3b26',
                            color: '#f4e6cd'
                        }}
                    >Update user</Button>
                    <Button
                        disabled={username === "admin"}
                        variant="contained"
                        style={{
                            backgroundColor: '#2f3b26',
                            color: '#f4e6cd'
                        }}
                        onClick={handleDeleteUser}
                    >Delete user</Button>
                </Stack>
            </form>
        </Box>
    )
}