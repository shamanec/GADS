import { useState, useContext } from "react"
import { useNavigate } from "react-router-dom"
import { Auth } from "../../contexts/Auth"
import './Login.css'
import TextField from '@mui/material/TextField'
import Button from '@mui/material/Button'
import Alert from '@mui/material/Alert'
import { api } from '../../services/api.js'

export default function Login() {
    const [username, setUsername] = useState()
    const [password, setPassword] = useState()
    const { login } = useContext(Auth)
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
        }

        api.post(url, loginData)
            .then(response => {
                if (response.status !== 200) {
                    toggleAlert(response.data.error);
                    throw new Error(response.data.error)
                } else {
                    return response.data
                }
            })
            .then(json => {
                const sessionID = json.sessionID
                login(sessionID, json.username, json.role)
                navigate("/devices")
            })
            .catch(e => {
                console.log("Login failed")
                console.log(e)
            })
    }

    let gadsVersion = localStorage.getItem('gadsVersion') || 'unknown'

    return (
        <div className="top-wrapper">
            <div className="fancy-wrapper">
                <div id="funky-div">
                </div>
                <div className="login-wrapper">
                    <img
                        src="./images/gads.png"
                        style={{
                            width: '50%'
                        }}
                    ></img>
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
                                label="Username"
                                required
                                id="outlined-required"
                                style={{color: "#9ba984"}}
                                sx={{
                                    input: {
                                        background: "#9ba984"
                                    }
                                }}
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
                                style={{color: "#9ba984"}}
                            />
                        </label>
                        <div>
                            <Button
                                variant="contained"
                                type="submit"
                                style={{
                                    marginBottom: "5px",
                                    backgroundColor: "#2f3b26",
                                    color: "#9ba984",
                                    fontWeight: "bold"
                                }}
                            >Log in</Button>
                        </div>
                        {showAlert && <Alert severity="error">{alertText}</Alert>}
                    </form>
                    <p
                        style={{
                            width: '100%',
                            marginRight: '20px',
                            textAlign: 'right',
                            fontWeight: 'bold',
                            color: '#2f3b26'
                        }}
                    >{gadsVersion.startsWith('v') ? gadsVersion : "DEV"}
                    </p>
                </div>
            </div>
        </div>
    )
}