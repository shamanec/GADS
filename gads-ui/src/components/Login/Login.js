import { useState, useContext } from "react";
import { useNavigate } from "react-router-dom";
import { Auth } from "../../contexts/Auth";
import './Login.css'

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

    return (
        <div className="login-wrapper">
            <h1>GADS</h1>
            <h2>Please log in</h2>
            <form onSubmit={handleLogin}>
                <label>
                    <p>Username</p>
                    <input type="text" onChange={e => setUsername(e.target.value)} />
                </label>
                <label>
                    <p>Password</p>
                    <input type="password" onChange={e => setPassword(e.target.value)} />
                </label>
                <div>
                    <button type="submit">Log in</button>
                </div>
            </form>
        </div>
    )
}