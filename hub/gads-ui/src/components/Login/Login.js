import { useState, useContext } from "react"
import { useNavigate } from "react-router-dom"
import { Auth } from "../../contexts/Auth"
import './Login.css'
import TextField from '@mui/material/TextField'
import Button from '@mui/material/Button'
import Alert from '@mui/material/Alert'

export default function Login() {
    const [username, setUsername] = useState()
    const [password, setPassword] = useState()
    const [, , , login,] = useContext(Auth)
    const [showAlert, setShowAlert] = useState(false)
    const [alertText, setAlertText] = useState()
    const navigate = useNavigate()

    function toggleAlert(message) {
        setAlertText(message)
        setShowAlert(true)
    }

    function handleLogin(event) {
        event.preventDefault()

        let url = `/authenticate`

        const loginData = {
            username: username,
            password: password,
        };

        fetch(url, {
            method: 'POST',
            body: JSON.stringify(loginData)
        })
            .then((response) => {
                if (!response.ok) {
                    return response.json().then((json) => {
                        toggleAlert(json.error);
                        throw new Error(json.error)
                    });
                } else {
                    return response.json().then((json) => {
                        return json;
                    });
                }
            })
            .then(json => {
                const sessionID = json.sessionID
                login(sessionID, json.username, json.role)
                navigate("/devices")
            })
            .catch((e) => {
                console.log(e)
            })
    }

    return (
        <div className="top-wrapper">
            <div className="fancy-wrapper">
                <div id="funky-div">
                </div>
                <div className="login-wrapper">
                    <h1>GADS</h1>
                    <h2>Please log in</h2>
                    <form
                        onSubmit={handleLogin}
                        style={{
                            display: "flex",
                            flexDirection: "column"
                        }}
                    >
                        <label>
                            <TextField
                                onChange={e => setUsername(e.target.value)}
                                label="Email"
                                required
                                id="outlined-required"
                            />
                        </label>
                        <label
                            style={{
                                marginTop: "20px",
                                marginBottom: "20px"
                            }}
                        >
                            <TextField
                                onChange={e => setPassword(e.target.value)}
                                type="password"
                                label="Password"
                                required
                                id="outlined-required"
                            />
                        </label>
                        <div>
                            <Button
                                variant="contained"
                                type="submit"
                                style={{
                                    marginBottom: "5px",
                                    backgroundColor: "#265ed3"
                                }}
                            >Log in</Button>
                        </div>
                        {showAlert && <Alert severity="error">{alertText}</Alert>}
                    </form>
                </div>
            </div>
        </div>
    )
}