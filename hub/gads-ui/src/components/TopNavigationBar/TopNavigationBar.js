import { useContext, useState } from 'react'
import './TopNavigationBar.css'
import { NavLink } from 'react-router-dom'
import { Auth } from '../../contexts/Auth'
import Button from '@mui/material/Button'
import { api } from '../../services/api.js'

export default function NavBar() {
    const {username} = useContext(Auth)

    const [showAdmin, setShowAdmin] = useState(false)

    const roleFromStorage = localStorage.getItem('userRole')

    if (roleFromStorage == 'admin') {
        if (!showAdmin) {
            setShowAdmin(true)
        }
    }

    return (
        <div
            className='navbar-wrapper'
        >
            <img
                src='./images/no-gads.png'
                alt='gads-logo'
                style={{
                    width: '50px',
                    marginLeft: '10px'
                }}
            ></img>
            <nav
                className="navbar"
            >
                <StyledNavLink
                    to="/devices"
                    linkText='Devices'
                />
                {showAdmin && (
                    <StyledNavLink
                        to="/admin"
                        linkText='Admin'
                    />
                )}
            </nav>
            <div
                className="social-buttons-wrapper"
            >
                <p style={{ fontWeight: "bold"}}>Welcome, {username}</p>
                <KoFiButton></KoFiButton>
                <GithubButton></GithubButton>
                <DiscordButton></DiscordButton>
                <LogoutButton></LogoutButton>
            </div>
        </div>

    )
}

function StyledNavLink({ to, linkText }) {
    return (
        <NavLink className="nav-bar-link"
            style={({ isActive }) => ({
                backgroundColor: isActive ? "#0c111e" : "",
                color: isActive ? "#78866B" : "#0c111e",
                fontWeight: "bold"
            })}
            to={to}
        >
            {linkText}
        </NavLink>
    )
}

function GithubButton() {
    return (
        <a
            className='github-button'
            target='_blank'
            href='https://github.com/shamanec/GADS'
        >
            <img
                src='./images/github.png'
                alt='github icon'
            />
        </a>
    )
}

function DiscordButton() {
    return (
        <a
            className='discord-button'
            target='_blank'
            href='https://discordapp.com/users/365565274470088704'
        >
            <img
                src='./images/discord.png'
                alt='discord icon'
            />
        </a>
    )
}

function KoFiButton() {
    return (
        <a
            className='ko-fi-button'
            target='_blank'
            href='https://ko-fi.com/shamanec'
        >
            <img
                src='./images/kofi_s_logo_nolabel.png'
                alt='kofi icon'
            />
        </a>
    )
}

function LogoutButton() {
    const {logout} = useContext(Auth)
    let url = `/logout`

    function handleLogout() {
        api.post(url)
            .then(response => {
                if (response.status !== 200) {
                    logout()
                    throw new Error('Network response was not ok.');
                }
                logout()
            })
            .catch(e => {
                console.log(e)
            })
    }
    return (
        <Button
            variant="contained"
            type="submit"
            onClick={handleLogout}
            style={{
                marginLeft: "20px",
                backgroundColor: "#914400",
                marginTop: "5px",
                marginBottom: "5px",
                color: "#0c111e",
                fontWeight: "bold"
            }}
        >Logout</Button>
    )
}