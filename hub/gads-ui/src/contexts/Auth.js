import { createContext, useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useSnackbar } from './SnackBarContext'
import { api } from '../services/api'

export const Auth = createContext()

export const AuthProvider = ({ children }) => {
    const { showSnackbar } = useSnackbar()
    const navigate = useNavigate()

    const [accessToken, setAccessToken] = useState('')
    const [userName, setUserName] = useState('')
    const [userRole, setUserRole] = useState('')
    const [loading, setLoading] = useState(true)

    // Function to fetch user info from JWT
    const fetchUserInfo = async (token) => {
        try {
            // Only proceed if we have a token
            if (!token) {
                setLoading(false)
                return
            }
            
            const response = await api.get('/user-info')
            const userData = response.data
            
            setUserName(userData.username)
            setUserRole(userData.role)
            localStorage.setItem('userRole', userData.role)
            localStorage.setItem('username', userData.username)
        } catch (error) {
            console.error('Failed to fetch user info:', error)
            // If we can't get user info from JWT, clear the token
            logout()
        } finally {
            setLoading(false)
        }
    }

    function login(token) {
        setAccessToken(token)
        localStorage.setItem('accessToken', token)
        // Fetch user info right after login
        fetchUserInfo(token)
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
            fetchUserInfo(storedToken)
        } else {
            setLoading(false)
        }
    }, [])


    return <Auth.Provider value={{ 
        accessToken, 
        userName, 
        userRole, 
        login, 
        logout, 
        loading 
    }}>
        {children}
    </Auth.Provider>
}