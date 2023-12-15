import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField'
import { useState, useContext } from 'react';
import { Button } from '@mui/material';
import { Alert } from '@mui/material';
import MenuItem from '@mui/material/MenuItem';
import './AddUser.css'
import Select from '@mui/material/Select';
import { Auth } from '../../contexts/Auth';


export default function AddUser() {
    const [authToken, ,] = useContext(Auth)

    // Inputs
    const [username, setUsername] = useState()
    const [password, setPassword] = useState()
    const [role, setRole] = useState('user')
    const [email, setEmail] = useState()

    // Submission button
    const [buttonDisabled, setButtonDisabled] = useState(true)

    // Alert
    const [showAlert, setShowAlert] = useState(false)
    const [alertText, setAlertText] = useState()
    const [alertSeverity, setAlertSeverity] = useState('error')

    // Validations
    const [emailValid, setEmailValid] = useState(false)
    const [passwordValid, setPasswordValid] = useState(false)

    // Form styles
    const [emailColor, setEmailColor] = useState('')
    const [passwordColor, setPasswordColor] = useState('')

    function showAlertWithTimeout(alertText, severity) {
        setAlertText(alertText)
        setShowAlert(true)
        setAlertSeverity(severity)

        setTimeout(() => {
            setShowAlert(false);
        }, 3000);
    }

    function validateEmail(email) {
        if (/^\w+([\.-]?\w+)*@\w+([\.-]?\w+)*(\.\w{2,4})+$/.test(email)) {
            setEmailColor('success')
            setEmailValid(true)
            setButtonDisabled(!passwordValid)
        } else {
            setEmailColor('error')
            setEmailValid(false)
            setButtonDisabled(true)
        }
    }

    function validatePassword(password) {
        if (/^(?=.*[A-Za-z])(?=.*\d)(?=.*[@$!%*#?&])[A-Za-z\d@$!%*#?&]{8,}$/.test(password)) {
            setPasswordColor('success')
            setPasswordValid(true)
            setButtonDisabled(!emailValid)
        } else {
            setPasswordColor('error')
            setPasswordValid(false)
            setButtonDisabled(true)
        }
    }

    function handleAddUser(event) {
        event.preventDefault()

        if (!emailValid || !passwordValid) {
            showAlertWithTimeout('Invalid input', 'error')
            return
        }

        let url = `http://${process.env.REACT_APP_GADS_BACKEND_HOST}/admin/user`

        const loginData = {
            username: username,
            password: password,
            role: role,
            email: email
        };

        fetch(url, {
            method: 'POST',
            body: JSON.stringify(loginData),
            headers: {
                'X-Auth-Token': authToken
            }
        })
            .then((response) => {
                if (!response.ok) {
                    return response.json().then((json) => {
                        showAlertWithTimeout(json.error, 'error')
                        throw new Error('Network response was not ok.');
                    });
                }
                showAlertWithTimeout('Successfully added user', 'success')
            })
            .catch((e) => {
                console.log(e)
            })
    }

    return (
        <div>

            <form onSubmit={handleAddUser}>

                <Stack className='add-user' alignItems="center" justifyContent="center" spacing={2}>
                    <h3>Add user</h3>
                    <label style={{}}>
                        <TextField
                            onChange={e => setEmail(e.target.value)}
                            label="Email"
                            color={emailColor}
                            required
                            id="outlined-required"
                            autoComplete='off'
                            onKeyUp={e => validateEmail(e.target.value)}
                        />
                    </label>
                    <label style={{}}>
                        <TextField
                            onChange={e => setPassword(e.target.value)}
                            label="Password"
                            color={passwordColor}
                            required
                            id="outlined-required"
                            autoComplete='off'
                            onKeyUp={e => validatePassword(e.target.value)}
                        />
                    </label>
                    <label>
                        <TextField
                            onChange={e => setUsername(e.target.value)}
                            label="Username"
                            autoComplete='off'
                        />
                    </label>
                    <Select
                        defaultValue='user'
                        value={role}
                        onChange={(event) => setRole(event.target.value)}
                        style={{ width: '223px', marginTop: "20px" }}
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
                        >Add user</Button>
                    </div>
                    {showAlert && <Alert id="add-user-alert" severity={alertSeverity}>{alertText}</Alert>}
                </Stack >
            </form >
        </div>
    )
}