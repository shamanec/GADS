import './TopNavigationBar.css'
import { NavLink } from 'react-router-dom'

export default function NavBar() {
    return (
        <div className='navbar-wrapper'>
            <nav className="navbar">
                <StyledNavLink to="/" linkText='Home' />
                <StyledNavLink to="/devices" linkText='Devices' />
                <StyledNavLink to="/logs" linkText='Logs' />
            </nav>
            <div className="social-buttons-wrapper">
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