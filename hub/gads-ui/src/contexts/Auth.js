import { createContext, useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'

export const Auth = createContext()

export const AuthProvider = ({ children }) => {
    const [authToken, setAuthToken] = useState('')
    const [userName, setUserName] = useState('')
    const [userRole, setUserRole] = useState('')
    const navigate = useNavigate()

    function login(token, name, role) {
        setAuthToken(token)
        setUserName(name)
        setUserRole(role)
        localStorage.setItem('authToken', token)
        localStorage.setItem('userRole', role)
        localStorage.setItem('username', name)
    }

    function logout() {
        setAuthToken(null)
        setUserName('')
        setUserRole('')
        localStorage.removeItem('authToken')
        localStorage.removeItem('userRole')
        localStorage.removeItem('username')
    }

    useEffect(() => {
        // Redirect to "/" on logout (when authToken becomes null)
        // We have to do it in useEffect because AuthProvider is not a component so we need a hook
        if (authToken === null) {
            navigate('/')
        }
    }, [authToken, navigate])

    useEffect(() => {
        // Check if the auth token exists in localStorage on initial load
        const storedToken = localStorage.getItem('authToken')
        if (storedToken) {
            setAuthToken(storedToken)
        }
        const storedUsername = localStorage.getItem('username')
        if (storedUsername) {
            setUserName((storedUsername))
        }
    }, [])


    return <Auth.Provider value={{ authToken, userName, userRole, login, logout }}>{children}</Auth.Provider>
}