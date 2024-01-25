import { useContext } from 'react'
import { Routes, Route } from 'react-router-dom'

import './Gads.css'

import { Header } from './components/Header'

import Login from './pages/Login'
import Admin from './pages/Admin'
import DeviceSelection from './components/DeviceSelection/DeviceSelection' //TODO: Remove from components and create corresponding page
import DeviceControl from './components/DeviceControl/DeviceControl' //TODO: Remove from components and create corresponding page
import { Auth } from './contexts/Auth'

function Gads() {
  const { authToken, user } = useContext(Auth)

  if (!authToken) {
    return <Login />
  }

  return (
    <div style={{ backgroundColor: "#273616", height: "100%" }}>
      <Header user={user} />
      <Routes>
        <Route path="/devices" element={<DeviceSelection />} />
        <Route path="/devices/control/:id" element={<DeviceControl />} />
        <Route path="/admin" element={<Admin />} />
      </Routes>
    </div>
  );
}

export default Gads;
