import { createContext, useState, useEffect } from "react"

export const Auth = createContext()

export const AuthProvider = ({ children }) => {
    const [authToken, setAuthToken] = useState(localStorage.getItem('authToken') || '')

    function login(token) {
        setAuthToken(token)
        localStorage.setItem('authToken', token);
    }

    function logout() {
        setAuthToken(null);
        localStorage.removeItem('authToken');
    }

    useEffect(() => {
        // Check if the auth token exists in localStorage on initial load
        const storedToken = localStorage.getItem('authToken');
        if (storedToken) {
            setAuthToken(storedToken);
        }
    }, []);


    return <Auth.Provider value={[authToken, login, logout]}>{children}</Auth.Provider>;
}