import { useContext } from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import { FiX, FiSettings } from 'react-icons/fi'

import { api } from '../../services/axios'

import { Auth } from '../../contexts/Auth'

import styles from './styles.module.scss'

export function Header({ user }) {
    const { authToken, logout } = useContext(Auth)

    const location = useLocation()

    const handleLogout = async () => {
        await api.post('/logout', null, {
            headers: {
                'X-Auth-Token': authToken
            }
        }).then(res => {
            if(res.status === 200) {
                return logout()
            }
            
            logout()
        })
    }

    return(
        <header className={styles.headerContainer}>
            <div className={styles.headerContent}>
                <img src="./images/gads-concept.svg" alt="gads icon" />
                <nav>
                    <NavLink to={'/devices'} className={`${location.pathname === '/devices' && styles.active}`}>
                        <span>Devices</span>
                    </NavLink>
                </nav>

                <div className={styles.rightElements}>
                    <NavLink to={'/admin'} className={`${location.pathname === '/admin' && styles.active}`} style={{padding: '10px 14px', borderRadius: '8px'}}>
                        <FiSettings color='var(--gray-500)' />
                    </NavLink>
                    <button onClick={handleLogout}>
                        <FiX color='var(--white)' />
                        {user.username}
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