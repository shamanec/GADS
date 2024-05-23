import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField'
import { useState, useContext } from 'react';
import { Button } from '@mui/material';
import { Alert } from '@mui/material';
import MenuItem from '@mui/material/MenuItem';
import './AddUser.css'
import Select from '@mui/material/Select';
import { Auth } from '../../../contexts/Auth';
import { api } from '../../../services/api.js'


export default function AddUser() {
    const [authToken] = useContext(Auth)

    // Inputs
    const [username, setUsername] = useState()
    const [password, setPassword] = useState()
    const [role, setRole] = useState('user')

    // Submission button
    const [buttonDisabled, setButtonDisabled] = useState(true)

    // Alert
    const [showAlert, setShowAlert] = useState(false)
    const [alertText, setAlertText] = useState()
    const [alertSeverity, setAlertSeverity] = useState('error')

    // Validations
    const [passwordValid, setPasswordValid] = useState(false)
    const [usernameValid, setUsernameValid] = useState(false)

    // Form styles
    const [usernameColor, setUsernameColor] = useState('')
    const [passwordColor, setPasswordColor] = useState('')

    function showAlertWithTimeout(alertText, severity) {
        setAlertText(alertText)
        setShowAlert(true)
        setAlertSeverity(severity)

        setTimeout(() => {
            setShowAlert(false);
        }, 3000);
    }

    function validatePassword(password) {
        if (/^(?=.*[A-Za-z])(?=.*\d)(?=.*[@$!%*#?&])[A-Za-z\d@$!%*#?&]{6,}$/.test(password)) {
            setPasswordColor('success')
            setPasswordValid(true)
            setButtonDisabled(!usernameValid)

        } else {
            setPasswordColor('error')
            setPasswordValid(false)
            setButtonDisabled(true)
        }
    }

    function validateUsername(username) {
        if (/^[a-zA-Z0-9._-]{4,}$/.test(username)) {
            setUsernameColor('success')
            setUsernameValid(true)
            setButtonDisabled(!passwordValid)

        } else {
            setUsernameColor('error')
            setUsernameValid(false)
            setButtonDisabled(true)
        }
    }

    function handleAddUser(event) {
        event.preventDefault()

        if (!usernameValid || !passwordValid) {
            showAlertWithTimeout('Invalid input', 'error')
            return
        }

        let url = `/admin/user`

        const loginData = {
            username: username,
            password: password,
            role: role
        };

        api.post(url, loginData)
            .then(response => {
                if (response.status !== 200) {
                    return response.data.then(json => {
                        showAlertWithTimeout(json.error, 'error')
                    });
                }
                showAlertWithTimeout('Successfully added user', 'success')
            })
            .catch(e => {
                console.log(e)
            })
    }

    return (
        <div>

            <form onSubmit={handleAddUser}>

                <Stack
                    className='add-user'
                    alignItems="center"
                    justifyContent="center"
                    spacing={2}
                >
                    <h3>Add user</h3>
                    <label style={{ width: "80%"}}>
                        <TextField
                            onChange={e => setUsername(e.target.value)}
                            label="Username"
                            autoComplete='off'
                            required
                            color={usernameColor}
                            id="outlined-required"
                            onInput={e => validateUsername(e.target.value)}
                            helperText='Username should be at least 4 characters'
                        />
                    </label>
                    <label style={{ width: "80%"}}>
                        <TextField
                            onChange={e => setPassword(e.target.value)}
                            label="Password"
                            color={passwordColor}
                            required
                            id="outlined-required"
                            autoComplete='off'
                            onKeyUp={e => validatePassword(e.target.value)}
                            helperText='Password should be minimum 6 chars long, contain upper and lower case letter, digit and special char'
                        />
                    </label>
                    <Select
                        defaultValue='user'
                        value={role}
                        onChange={(event) => setRole(event.target.value)}
                        style={{width: '223px', marginTop: "20px"}}
                    >
                        <MenuItem value='user'>User</MenuItem>
                        <MenuItem value='admin'>Admin</MenuItem>
                    </Select>
                    <div>
                        <Button
                            variant="contained"
                            type="submit"
                            style={{}}
                            disabled={buttonDisabled}
                            style={{
                                backgroundColor: "#0c111e",
                                color: "#78866B",
                                fontWeight: "bold"
                            }}
                        >Add user</Button>
                    </div>
                    {showAlert && <Alert id="add-user-alert" severity={alertSeverity}>{alertText}</Alert>}
                </Stack>
            </form>
        </div>
    )
}