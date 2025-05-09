import { useState, useContext } from 'react'
import { useNavigate } from 'react-router-dom'
import { Auth } from '../../contexts/Auth'
import TextField from '@mui/material/TextField'
import Button from '@mui/material/Button'
import { api } from '../../services/api.js'
import { Box, Stack } from '@mui/material'
import { useSnackbar } from '../../contexts/SnackBarContext.js'

export default function Login() {
    const { showSnackbar, hideSnackbar } = useSnackbar()
    const { login } = useContext(Auth)
    const navigate = useNavigate()

    const [username, setUsername] = useState('')
    const [password, setPassword] = useState('')

    let gadsVersion = localStorage.getItem('gadsVersion') || 'unknown'

    function handleLogin(event) {
        event.preventDefault()

        const loginData = {
            username: username,
            password: password,
        }

        let url = `/authenticate`
        api.post(url, loginData)
            .then(response => {
                const json = response.data
                const sessionID = json.sessionID
                login(sessionID, json.username, json.role)
                hideSnackbar()
                navigate('/devices')

                return response.data
            })
            .catch((e) => {
                if (e.response) {
                    if (e.response.status === 401) {
                        showLoginError('Invalid credentials!')
                    }
                } else {
                    showLoginError('Something went wrong!')
                }
            })
    }

    // Use the custom snackbar to show an error with different messages
    const showLoginError = (message) => {
        showSnackbar({
            message: message,
            severity: 'error',
            duration: 3000,
        })
    }

    return (
        <Box style={{ height: '100vh', width: '100vw', justifyContent: 'center', alignItems: 'center', display: 'flex', backgroundColor: '#f4e6cd' }}>
            <Box
                style={{
                    width: '30%',
                    height: '500px',
                    backgroundColor: '#9ba984',
                    display: 'flex',
                    flexDirection: 'row',
                    borderRadius: '10px'
                }}
            >
                <Box
                    style={{
                        background: 'linear-gradient(62deg, rgba(38,199,127,1) 0%, rgba(200,137,0,1) 100%)',
                        width: '50%',
                        borderTopLeftRadius: '10px',
                        borderBottomLeftRadius: '10px'
                    }}
                ></Box>
                <Box
                    style={{
                        display: 'flex',
                        flexDirection: 'column',
                        justifyContent: 'center',
                        alignItems: 'center',
                        width: '50%'
                    }}
                >
                    <img
                        src='./images/gads.png'
                        style={{
                            width: '50%',
                            marginBottom: '20px'
                        }}
                    ></img>
                    <form onSubmit={handleLogin}>
                        <Stack spacing={2}>
                            <TextField
                                required
                                label='Username'
                                autoComplete='off'
                                size='small'
                                onChange={(e) => setUsername(e.target.value)}
                            />
                            <TextField
                                required
                                label='Password'
                                autoComplete='off'
                                size='small'
                                type='password'
                                onChange={(e) => setPassword(e.target.value)}
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
                            >Log In</Button>
                            <p
                                style={{
                                    width: '100%',
                                    marginRight: '20px',
                                    textAlign: 'right',
                                    fontWeight: 'bold',
                                    color: '#2f3b26',
                                }}
                            >{gadsVersion.startsWith('v') ? gadsVersion : 'DEV'}
                            </p>
                        </Stack>
                    </form>
                </Box>

            </Box>
        </Box>
    )
}