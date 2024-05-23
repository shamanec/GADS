import './Gads.css'
import DeviceSelection from './components/DeviceSelection/DeviceSelection'
import { Routes, Route } from 'react-router-dom'
import NavBar from './components/TopNavigationBar/TopNavigationBar'
import DeviceControl from './components/DeviceControl/DeviceControl'
import Login from './components/Login/Login'
import { useContext } from 'react'
import { Auth } from './contexts/Auth'
import AdminDashboard from './components/Admin/AdminDashboard'
import axiosInterceptor from './services/axiosInterceptor'

function Gads() {
  const {authToken, logout} = useContext(Auth)
  // Set the logout function from the Auth context on the axiosInterceptor to automatically logout on each 401
  axiosInterceptor(logout)

  if (!authToken) {
    return <Login />
  }

  return (
    <div style={{ backgroundColor: "#0c111e", height: "100%" }}>
      <NavBar />
      <Routes>
        <Route path="/devices" element={<DeviceSelection />} />
        <Route path="/devices/control/:id" element={<DeviceControl />} />
        <Route path="/admin" element={<AdminDashboard />} />
      </Routes>
    </div>
  );
}

export default Gads
