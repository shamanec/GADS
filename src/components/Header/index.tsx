import { FiBell, FiSettings } from 'react-icons/fi'
import { SignInButton } from '../SignInButton'
import { ActiveLink } from '../ActiveLink'

import styles from './styles.module.scss'

export function Header() {
    return(
        <header className={styles.headerContainer}>
            <div className={styles.headerContent}>
                <img src='/gads.svg' alt='GADS icon' />
                <nav>
                    <ActiveLink activeClassName={styles.active} href='/dashboard/devices'>
                        <a>Devices</a>
                    </ActiveLink>
                    <ActiveLink activeClassName={styles.active} href='/dashboard/logs'>
                        <a>Logs</a>
                    </ActiveLink>
                </nav>

                <div className={styles.rightElements}>
                    <a className={styles.githubButton} target='_blank' href='https://github.com/shamanec/GADS/tree/nextjs'>
                        <img src='/images/github.svg' alt='github icon' />
                        <span>GitHub</span>
                    </a>
                </div>
            </div>
        </header>
    )
}