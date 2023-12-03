import { useState, useContext } from "react";
import { useNavigate } from "react-router-dom";
import { Auth } from "../../contexts/Auth";
import './Login.css'
import TextField from '@mui/material/TextField';

export default function Login() {
    const [username, setUsername] = useState();
    const [password, setPassword] = useState();
    const [session, setSession] = useContext(Auth);
    const navigate = useNavigate()

    function handleLogin(event) {
        event.preventDefault()

        let url = `http://${process.env.REACT_APP_GADS_BACKEND_HOST}/authenticate`

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
                    throw new Error('Network response was not ok.');
                }
                // Parse the JSON data
                return response.json();
            })
            .then(json => {
                const sessionID = json.sessionID
                setSession(sessionID)
                navigate("/devices")
            })
            .catch((e) => {
                console.log(e)
            })
    }
    let img_src = './images/sea-wave.png'

    return (
        <div className="top-wrapper">
            <div className="fancy-wrapper">
                <div id="funky-div">
                </div>
                <div className="login-wrapper">
                    <h1>GADS</h1>
                    <h2>Please log in</h2>
                    <form onSubmit={handleLogin} style={{ display: "flex", flexDirection: "column" }}>
                        <label>
                            <TextField
                                onChange={e => setUsername(e.target.value)}
                                label="username"
                                required
                                id="outlined-required"
                                helperText="Username or email"
                            />
                        </label>
                        <label style={{ marginTop: "10px", marginBottom: "20px" }}>
                            <TextField
                                onChange={e => setPassword(e.target.value)}
                                type="password"
                                label="password"
                                required
                                id="outlined-required"
                            />
                        </label>
                        <div>
                            <button className="login-button" type="submit">Log in</button>
                        </div>
                    </form>
                </div>
            </div>
        </div>
    )
}