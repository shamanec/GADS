import './Gads.css'
import DeviceSelection from './components/DeviceSelection/DeviceSelection'
import { Routes, Route, Navigate } from 'react-router-dom'
import NavBar from './components/TopNavigationBar/TopNavigationBar'
import DeviceControl from './components/DeviceControl/DeviceControl'
import Login from './components/Login/Login'
import { useContext, useEffect } from 'react'
import { Auth } from './contexts/Auth'
import AdminDashboard from './components/Admin/AdminDashboard'
import axiosInterceptor from './services/axiosInterceptor'
import { DialogProvider } from './contexts/DialogContext'
import { SnackbarProvider } from './contexts/SnackBarContext'
import { LoadingOverlayProvider } from './contexts/LoadingOverlayContext'

function Gads() {
    const { accessToken, logout } = useContext(Auth)
    // Set the logout function from the Auth context on the axiosInterceptor to automatically logout on each 401
    axiosInterceptor(logout)

    useEffect(() => {
        localStorage.removeItem('gadsVersion')
        let version = process.env.REACT_APP_VERSION || 'unknown'
        localStorage.setItem('gadsVersion', version)
    }, [])

    if (!accessToken) {
        return <Login />
    }

    return (
        <div style={{ backgroundColor: "#f4e6cd", height: "100%" }}>
            <NavBar />
            <DialogProvider>
                <SnackbarProvider>
                    <Routes>
                        <Route path="/" element={<Navigate to="/devices" />} />
                        <Route path="/devices" element={<DeviceSelection />} />

                        <Route path="/devices/control/:udid" element={
                            <LoadingOverlayProvider>
                                <DeviceControl />
                            </LoadingOverlayProvider>
                        } />
                        <Route path="/admin" element={<AdminDashboard />} />
                    </Routes>
                </SnackbarProvider>
            </DialogProvider>
        </div>
    )
}

export default Gads
