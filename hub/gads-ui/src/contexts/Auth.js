import { createContext, useState, useEffect } from "react"

export const Auth = createContext()

export const AuthProvider = ({ children }) => {
    const [authToken, setAuthToken] = useState(localStorage.getItem('authToken') || '')
    const [userName, setUserName] = useState("")
    const [userRole, setUserRole] = useState("")

    function login(token, name, role) {
        setAuthToken(token)
        setUserName(name)
        setUserRole(role)
        localStorage.setItem('authToken', token);
        localStorage.setItem('userRole', role)
    }

    function logout() {
        setAuthToken(null);
        setUserName("")
        setUserRole("")
        localStorage.removeItem('authToken');
        localStorage.removeItem('userRole')
    }

    useEffect(() => {
        // Check if the auth token exists in localStorage on initial load
        const storedToken = localStorage.getItem('authToken');
        if (storedToken) {
            setAuthToken(storedToken);
        }
    }, []);


    return <Auth.Provider value={{ authToken, userName, userRole, login, logout }}>{children}</Auth.Provider>;
}