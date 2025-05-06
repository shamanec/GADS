import { createContext, useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useSnackbar } from './SnackBarContext'

export const Auth = createContext()

export const AuthProvider = ({ children }) => {
    const { showSnackbar } = useSnackbar()
    const navigate = useNavigate()

    const [accessToken, setAccessToken] = useState('')
    const [userName, setUserName] = useState('')
    const [userRole, setUserRole] = useState('')

    function login(token, name, role) {
        setAccessToken(token)
        setUserName(name)
        setUserRole(role)
        localStorage.setItem('accessToken', token)
        localStorage.setItem('userRole', role)
        localStorage.setItem('username', name)
    }

    function logout() {
        setAccessToken(null)
        setUserName('')
        setUserRole('')
        localStorage.removeItem('accessToken')
        localStorage.removeItem('userRole')
        localStorage.removeItem('username')
        showLogoutError()
    }

    const showLogoutError = () => {
        showSnackbar({
            message: 'You are logged out!',
            severity: 'warning',
            duration: 3000,
        })
    }

    useEffect(() => {
        // Redirect to "/" on logout (when accessToken becomes null)
        // We have to do it in useEffect because AuthProvider is not a component so we need a hook
        if (accessToken === null) {
            navigate('/')
        }
    }, [accessToken, navigate])

    useEffect(() => {
        // Check if the access token exists in localStorage on initial load
        const storedToken = localStorage.getItem('accessToken')
        if (storedToken) {
            setAccessToken(storedToken)
        }
        const storedUsername = localStorage.getItem('username')
        if (storedUsername) {
            setUserName((storedUsername))
        }
    }, [])


    return <Auth.Provider value={{ accessToken, userName, userRole, login, logout }}>{children}
    </Auth.Provider>
}