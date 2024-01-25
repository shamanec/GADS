import { useState, useContext } from 'react'
import { useNavigate } from 'react-router-dom'
import { Badge } from '../../components/Badge'

import { Auth } from '../../contexts/Auth'
import { api } from '../../services/axios'

import styles from '../../styles/Auth.module.scss'

export default function Login() {
    const { login } = useContext(Auth)

    const [username, setUsername] = useState('')
    const [password, setPassword] = useState('')

    const [message, setMessage] = useState({ visible: false, message: ''})
    const [isLoading, setIsLoading] = useState(false)
    
    const navigate = useNavigate()

    const handleSignIn = async (e) => {
        e.preventDefault()

        if(username.trim() === '' && password.trim() === '') {
            return setMessage({ visible: true, message: 'Please, enter with your credentials'})
        }

        if(username.trim() === '') {
            return setMessage({ visible: true, message: 'Please, enter a valid username' })
        }

        if(password.trim() === '') {
            return setMessage({ visible: true, message: 'Please, enter a valid password' })
        }

        setIsLoading(true)

        await api.post('/authenticate', {
            username,
            password
        }).then(res => {
            if(res.status === 200) {
                console.log(res.data)
                login(res.data.sessionID, res.data.username, res.data.role)
                navigate('/devices')

                setMessage({ visible: false, message: '' })
            } else {
                setMessage({ visible: true, message: 'An error ocurred, please try again.'})
            }
        })

        setIsLoading(false)
    }

    return (
        <div className={styles.mainContainer}>
            <aside className={styles.aside}>
                <p>Simple device farm for remote control of devices and Appium test execution on iOS/Android</p>

                <div className={styles.rowIcons}>
                    <a href='https://github.com/shamanec/GADS' target='_blank'>
                        <img src='./images/github.svg' alt='github icon' />
                    </a>
                    <a href='https://discordapp.com/users/365565274470088704' target='_blank'>
                        <img src='./images/discord.svg' alt='github icon' />
                    </a>
                </div>
            </aside>
            <main className={styles.mainSection}>
                <form className={styles.formContainer}>
                    <div className={styles.textAndSupportingText}>
                        <h2>Please log in</h2>
                        <span>Start control your devices</span>
                    </div>
                    <div className={styles.content}>
                        {message.visible && <Badge type='error' baseText='Error' contentText={message.message} />}
                        <div className={styles.inputGroup}>
                            <label htmlFor='email'>Email</label>
                            <input className={`${message.visible && message.message.includes('Please') && styles.error}`} type='email' name='email' id='email' placeholder='Enter your email' value={username} onChange={e => setUsername(e.target.value)} />
                        </div>
                        <div className={styles.inputGroup}>
                            <label htmlFor='password'>Password</label>
                            <input className={`${message.visible && message.message.includes('Please') && styles.error}`} type='password' name='password' id='password' placeholder='Enter your password' value={password} onChange={e => setPassword(e.target.value)} />
                        </div>
                    </div>
                    <button className={`${isLoading && styles.loading}`} onClick={handleSignIn}>
                        <span>Log in</span>
                    </button>
                </form>
            </main>
        </div>
    )
}