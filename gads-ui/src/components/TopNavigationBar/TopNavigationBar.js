import { useContext } from 'react'
import './TopNavigationBar.css'
import { NavLink } from 'react-router-dom'
import { Auth } from '../../contexts/Auth'

export default function NavBar() {
    return (
        <div className='navbar-wrapper'>
            <nav className="navbar">
                <StyledNavLink to="/" linkText='Home' />
                <StyledNavLink to="/devices" linkText='Devices' />
                <StyledNavLink to="/logs" linkText='Logs' />
            </nav>
            <div className="social-buttons-wrapper">
                <LogoutButton></LogoutButton>
                <GithubButton></GithubButton>
                <DiscordButton></DiscordButton>
            </div>
        </div>

    )
}

function StyledNavLink({ to, linkText }) {
    return (
        <NavLink className="nav-bar-link"
            style={({ isActive }) => ({
                backgroundColor: isActive ? "#273616" : "",
                color: "#E0D8C0",
            })}
            to={to}
        >
            {linkText}
        </NavLink>
    )
}

function GithubButton() {
    return (
        <a className='github-button' target='_blank' href='https://github.com/shamanec/GADS'>
            <img src='./images/github.png' alt='github icon' />
        </a>
    )
}

function DiscordButton() {
    return (
        <a className='discord-button' target='_blank' href='https://discordapp.com/users/365565274470088704'>
            <img src='./images/discord.png' alt='discord icon' />
        </a>
    )
}

function LogoutButton() {
    const [authToken, login, logout] = useContext(Auth)
    let url = `http://${process.env.REACT_APP_GADS_BACKEND_HOST}/logout`

    function handleLogout() {
        fetch(url, {
            method: 'POST',
            headers: {
                'X-Auth-Token': authToken
            }
        })
            .then((response) => {
                logout()
                if (!response.ok) {
                    throw new Error('Network response was not ok.');
                }
            })
            .catch((e) => {
                console.log(e)
            })
    }
    return (
        <button onClick={handleLogout}>Logout</button>
    )
}