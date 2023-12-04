import './Gads.css';
import DeviceSelection from './components/DeviceSelection/DeviceSelection';
import { Routes, Route } from 'react-router-dom';
import ProviderLogsTable from './components/ProviderLogsTable/ProviderLogsTable';
import NavBar from './components/TopNavigationBar/TopNavigationBar';
import Home from './components/Home/Home'
import DeviceControl from './components/DeviceControl/DeviceControl'
import Login from './components/Login/Login';
import { useContext } from 'react';
import { Auth } from './contexts/Auth';
import AdminDashboard from './components/Admin/AdminDashboard';

function Gads() {
  const [authToken] = useContext(Auth);

  if (!authToken) {
    return <Login />
  }

  return (
    <div style={{ backgroundColor: "#273616", height: "100vh" }}>
      <NavBar />
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/devices" element={<DeviceSelection />} />
        <Route path="/devices/control/:id" element={<DeviceControl />} />
        <Route path="/logs" element={<ProviderLogsTable />} />
        <Route path="/admin" element={<AdminDashboard />} />
      </Routes>
    </div>
  );
}

export default Gads;
