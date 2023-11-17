import './Gads.css';
import DeviceTable from './components/DeviceTable/DeviceTable';
import { Routes, Route } from 'react-router-dom';
import ProviderLogsTable from './components/ProviderLogsTable/ProviderLogsTable';
import NavBar from './components/TopNavigationBar/TopNavigationBar';
import Home from './components/Home/Home'
import DeviceControl from './components/DeviceControl/DeviceControl'

function Gads() {
  return (
    <div>
      <NavBar />
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/devices" element={<DeviceTable />} />
        <Route path="/devices/control/:id" element={<DeviceControl />} />
        <Route path="/logs" element={<ProviderLogsTable />} />
      </Routes>
    </div>
  );
}

export default Gads;
