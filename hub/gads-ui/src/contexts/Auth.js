import { createContext, useState, useEffect } from "react"

export const Auth = createContext()

export const AuthProvider = ({ children }) => {
    const [authToken, setAuthToken] = useState('')
    const [userName, setUserName] = useState("")
    const [userRole, setUserRole] = useState("")

    function login(token, name, role) {
        setAuthToken(token)
        setUserName(name)
        setUserRole(role)
        localStorage.setItem('authToken', token);
        localStorage.setItem('userRole', role)
        localStorage.setItem('username', name);
    }

    function logout() {
        setAuthToken(null);
        setUserName("")
        setUserRole("")
        localStorage.removeItem('authToken');
        localStorage.removeItem('userRole')
        localStorage.removeItem('username')
    }

    useEffect(() => {
        // Check if the auth token exists in localStorage on initial load
        const storedToken = localStorage.getItem('authToken');
        if (storedToken) {
            setAuthToken(storedToken);
        }
        const storedUsername = localStorage.getItem('username')
        if (storedUsername) {
            setUserName((storedUsername))
        }
    }, []);


    return <Auth.Provider value={{ authToken, userName, userRole, login, logout }}>{children}</Auth.Provider>;
}