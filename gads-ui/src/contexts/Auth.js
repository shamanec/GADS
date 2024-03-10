import { createContext, useState, useEffect } from 'react'

import { api } from '../services/api'

export const Auth = createContext()


export const AuthProvider = ({ children }) => {
    const [authToken, setAuthToken] = useState(localStorage.getItem('authToken') || '')
    const [user, setUser] = useState({
        username: '',
        role: ''
    })

    useEffect(() => {
        const storedToken = localStorage.getItem('authToken')
        const storedRole = localStorage.getItem('role')

        if(storedToken) {
            setAuthToken(storedToken)
            setUser({...user, role: storedRole})
        }
    }, [])

    async function signIn(username, password) {
        try {
            const response = await api.post('/authenticate', {
                username,
                password
            })
      
            if (response.status === 200) {
                const token = response.data.sessionID
      
                localStorage.setItem('authToken', token)
                localStorage.setItem('role', response.data.role)
      
                setUser({
                    username: response.data.username,
                    role: response.data.role
                })

                setAuthToken(token)

                api.defaults.headers['X-Auth-Token'] = `${token}`
      
                return {
                    success: true,
                    message: 'Login successfully.',
                    response: response
                }
            } else {
                return {
                    success: false,
                    message: 'An unknown error has occurred.',
                    response: response
                }
            }
        } catch (error) {
            if (error.response) {
              if (error.response.status === 401) {
                return {
                  success: false,
                  message: 'Invalid credentials. Check your email and password.',
                  response: error.response
                };
                } else if (error.response.status === 404) {
                    return {
                        success: false,
                        message: 'User not found',
                        response: error.response
                    }
                }
            }
      
            return {
                success: false,
                message: 'An unknown error has occurred.',
                response: error.response
            }
        }
    }

    async function signOut() {
        try {
            const response = await api.post('/logout', null)
      
            if (response.status === 200) {
                setAuthToken(null)
                setUser({ username: '', role: ''})

                localStorage.removeItem('authToken')
                localStorage.removeItem('role')
      
                delete api.defaults.headers['X-Auth-Token']
      
                return {
                    success: true,
                    message: 'Logout successfully.',
                    response: response
                }
            } else {
                return {
                    success: false,
                    message: 'An unknown error has occurred.',
                    response: response
                }
            }
        } catch (error) {      
            return {
                success: false,
                message: 'An unknown error has occurred.',
                response: error.response
            }
        }
    }

    return(
        <Auth.Provider value={{ user, authToken, signIn, signOut }}>
            {children}
        </Auth.Provider>
    )
}