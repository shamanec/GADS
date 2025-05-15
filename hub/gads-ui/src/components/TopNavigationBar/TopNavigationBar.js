import { useContext, useEffect, useState } from 'react'
import './TopNavigationBar.css'
import { NavLink } from 'react-router-dom'
import { Auth } from '../../contexts/Auth'
import Button from '@mui/material/Button'
import { api } from '../../services/api.js'
import Divider from '@mui/material/Divider'
import CircularProgress from '@mui/material/CircularProgress'

export default function NavBar() {
    const { userName, loading } = useContext(Auth)

    const [showAdmin, setShowAdmin] = useState(false)

    useEffect(() => {
        const roleFromStorage = localStorage.getItem('userRole')
        if (roleFromStorage === 'admin') {
            setShowAdmin(true)
        }
    }, [])

    let appVersion = localStorage.getItem('gadsVersion') || 'unknown'

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
                className='navbar'
            >
                <StyledNavLink
                    to='/devices'
                    linkText='Devices'
                />
                {showAdmin && (
                    <StyledNavLink
                        to='/admin'
                        linkText='Admin'
                    />
                )}
            </nav>
            <Divider
                orientation='vertical'
                flexItem
                style={{
                    marginLeft: '10px',
                    marginRight: '20px'
                }}
            />
            <div
                style={{
                    fontWeight: 'bold',
                    color: '#2f3b26',
                    pointerEvents: 'none',
                    userSelect: 'none',
                }}
            >{appVersion.startsWith('v') ? appVersion : 'DEV'}</div>
            <div
                className='social-buttons-wrapper'
            >
                <p style={{ fontWeight: 'bold' }}>Welcome, {userName}</p>
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
        <NavLink className='nav-bar-link'
            style={({ isActive }) => ({
                backgroundColor: isActive ? '#2f3b26' : '',
                color: isActive ? '#9ba984' : '#2f3b26',
                fontWeight: 'bold'
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
            href='https://discord.gg/5amWvknKQd'
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
    const { logout } = useContext(Auth)
    let url = `/logout`

    function handleLogout() {
        api.post(url)
            .then(response => {
                if (response.status !== 200) {
                    logout()
                    throw new Error('Network response was not ok.')
                }
                logout()
            })
            .catch(e => {
                console.log(e)
            })
    }

    return (
        <Button
            variant='contained'
            type='submit'
            onClick={handleLogout}
            style={{
                marginLeft: '20px',
                backgroundColor: '#dfbe82',
                marginTop: '5px',
                marginBottom: '5px',
                color: '#2f3b26',
                fontWeight: 'bold'
            }}
        >Logout</Button>
    )
}