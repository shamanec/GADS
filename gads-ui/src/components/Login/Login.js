import { useState } from "react";
import { useNavigate } from "react-router-dom";

export default function Login({ setAccessToken }) {
    const [username, setUsername] = useState();
    const [password, setPassword] = useState();
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
            body: JSON.stringify(loginData),
            credentials: 'include'
        })
            .then(response => {
                console.log('is success')
                console.log(response.headers.get('X-Auth-Token'))
                response.headers.forEach((value, name) => {
                    console.log(`${name}: ${value}`);
                });
                const authCookie = response.headers.get('Set-Cookie');
                console.log("COOKIE: " + authCookie)
                if (authCookie) {
                    console.log("setting access token")
                    document.cookie = authCookie;
                    setAccessToken = authCookie
                }
                navigate("/devices")
            })
            .catch((e) => {
                console.log(e)
            })
    }

    return (
        <div className="login-wrapper">
            <h1>Please Log In</h1>
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
                    <button type="submit">Submit</button>
                </div>
            </form>
        </div>
    )
}