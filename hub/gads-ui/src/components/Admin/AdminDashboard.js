import Tabs from '@mui/material/Tabs';
import Tab from '@mui/material/Tab';
import { useState } from "react";
import { Box } from "@mui/material";
import UsersAdministration from "./Users/UsersAdministration";
import FilesAdministration from "./Files/FilesAdministration";
import DevicesAdministration from './Devices/DevicesAdministration';
import ProvidersAdministration from './Providers/ProvidersAdministration';
import GlobalSettings from './GlobalSettings/GlobalSettings';
import WebRTCClient from './webrtc';


export default function AdminDashboard() {
    const [currentTabIndex, setCurrentTabIndex] = useState(0)

    const handleTabChange = (e, tabIndex) => {
        setCurrentTabIndex(tabIndex)
    }

    return (
        <Box
            id='dashboard-box'
            style={{
                width: '100%',
                height: '100%'
            }}
        >
            <Tabs
                value={currentTabIndex}
                onChange={handleTabChange}
                TabIndicatorProps={{
                    style: {
                        background: "#2f3b26",
                        height: "5px"
                    }
                }}
                textColor="#f4e6cd"
                sx={{
                    color: "#2f3b26",
                    fontFamily: "Verdana"
                }}
            >
                <Tab
                    label="Providers"
                    style={{
                        textTransform: 'none',
                        fontSize: '16px',
                        fontWeight: "bold"
                    }}
                />
                <Tab
                    label="Devices"
                    style={{
                        textTransform: 'none',
                        fontSize: '16px',
                        fontWeight: "bold"
                    }}
                />
                <Tab
                    label="Users"
                    style={{
                        textTransform: 'none',
                        fontSize: '16px',
                        fontWeight: "bold"
                    }}
                />
                <Tab
                    label="Files"
                    style={{
                        textTransform: 'none',
                        fontSize: '16px',
                        fontWeight: "bold"
                    }}
                />
                <Tab
                    label="Global Settings"
                    style={{
                        textTransform: 'none',
                        fontSize: '16px',
                        fontWeight: "bold"
                    }}
                />
                <Tab
                    label="OPALQ"
                    style={{
                        textTransform: 'none',
                        fontSize: '16px',
                        fontWeight: "bold"
                    }}
                />
            </Tabs>
            {currentTabIndex === 0 && <ProvidersAdministration></ProvidersAdministration>}
            {currentTabIndex === 1 && <DevicesAdministration></DevicesAdministration>}
            {currentTabIndex === 2 && <UsersAdministration></UsersAdministration>}
            {currentTabIndex === 3 && <FilesAdministration></FilesAdministration>}
            {currentTabIndex === 4 && <GlobalSettings></GlobalSettings>}
            {currentTabIndex === 5 && <WebRTCClient></WebRTCClient>}
        </Box >
    )
}