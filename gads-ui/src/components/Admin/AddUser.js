import Stack from '@mui/material/Stack';
import TextField from '@mui/material/TextField'
import { useState } from 'react';
import { Button } from '@mui/material';
import { Alert } from '@mui/material';
import MenuItem from '@mui/material/MenuItem';
import './AddUser.css'
import Select from '@mui/material/Select';


export default function AddUser() {
    const [username, setUsername] = useState()
    const [password, setPassword] = useState()
    const [role, setRole] = useState('user')
    const [email, setEmail] = useState()
    const [showAlert, setShowAlert] = useState(false)
    const [alertText, setAlertText] = useState()

    function handleAddUser(event) {
        event.preventDefault()

        let url = `http://${process.env.REACT_APP_GADS_BACKEND_HOST}/admin/user`

        const loginData = {
            username: username,
            password: password,
            role: role,
            email: email
        };

        fetch(url, {
            method: 'POST',
            body: JSON.stringify(loginData)
        })
            .then((response) => {
                if (!response.ok) {
                    return response.json().then((json) => {
                        toggleAlert(json.error);
                        throw new Error('Network response was not ok.');
                    });
                }
            })
            .catch((e) => {
                console.log(e)
            })
    }

    function toggleAlert(message) {
        setAlertText(message)
        setShowAlert(true)
    }

    return (
        <div>
            <form onSubmit={handleAddUser}>
                <Stack className='add-user' alignItems="center" justifyContent="center" spacing={2}>
                    <label autoCom>
                        <TextField
                            onChange={e => setUsername(e.target.value)}
                            label="username"
                            required
                            id="outlined-required"
                            autoComplete='off'
                        />
                    </label>
                    <label style={{}}>
                        <TextField
                            onChange={e => setPassword(e.target.value)}
                            label="password"
                            required
                            id="outlined-required"
                            autoComplete='off'
                        />
                    </label>
                    <Select
                        defaultValue='user'
                        value={role}
                        onChange={(event) => setRole(event.target.value)}
                        style={{ width: '100px', marginTop: "20px" }}
                    >
                        <MenuItem value='user'>User</MenuItem>
                        <MenuItem value='admin'>Admin</MenuItem>
                    </Select>
                    <label style={{}}>
                        <TextField
                            onChange={e => setEmail(e.target.value)}
                            label="email"
                            required
                            id="outlined-required"
                            autoComplete='off'
                        />
                    </label>
                    <div>
                        <Button
                            variant="contained"
                            type="submit"
                            style={{}}
                        >Add user</Button>
                    </div>
                    {showAlert && <Alert severity="error">{alertText}</Alert>}
                </Stack >
            </form >
        </div>
    )
}