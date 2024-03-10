import { useContext } from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import { FiX, FiSettings, FiLogOut } from 'react-icons/fi'
import { useNavigate } from 'react-router-dom'

import { Auth } from '../../contexts/Auth'

import styles from './styles.module.scss'

export function Header({ user }) {
    const navigate = useNavigate()

    const { signOut } = useContext(Auth)

    const location = useLocation()

    const handleLogout = async () => {
        const result = await signOut()

        if(result.response?.status === 200) {
            navigate('/login')
        } else {
            navigate('/login')
        }
    }

    return(
        <header className={styles.headerContainer}>
            <div className={styles.headerContent}>
                <img src="./images/gads-concept.svg" alt="gads icon" />
                <nav>
                    <NavLink to={'/devices'} className={`${location.pathname === '/devices' && styles.active}`}>
                        <span>Devices</span>
                    </NavLink>
                    {user.role === 'admin' && (
                        <NavLink to={'/admin'} className={`${location.pathname === '/admin' && styles.active}`}>
                            <span>Admin</span>
                        </NavLink>
                    )}
                </nav>

                <div className={styles.rightElements}>
                    <button onClick={handleLogout}>
                        <FiLogOut color='var(--white)' />
                        Log out
                    </button>
                    <div className={styles.socialIcons}>
                        <a href='https://github.com/shamanec/GADS' target='_blank'>
                            <img src='./images/github.svg' alt='github icon' />
                        </a>
                        <a href='https://discordapp.com/users/365565274470088704' target='_blank'>
                            <img src='./images/discord.svg' alt='github icon' />
                        </a>
                    </div>
                </div>
            </div>
        </header>
    )
}