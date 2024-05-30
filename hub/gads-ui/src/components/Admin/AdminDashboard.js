import Tabs from '@mui/material/Tabs';
import Tab from '@mui/material/Tab';
import { useState } from "react";
import { Box } from "@mui/material";
import UsersAdministration from "./Users/UsersAdministration";
import ProvidersAdministration from "./Providers/ProvidersAdministration";


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
                    label="User administration"
                    style={{
                        textTransform: 'none',
                        fontSize: '16px',
                        fontWeight: "bold"
                    }}
                />
                <Tab
                    label="Providers administration"
                    style={{
                        textTransform: 'none',
                        fontSize: '16px',
                        fontWeight: "bold"
                    }}
                />
            </Tabs>
            {currentTabIndex === 0 && <UsersAdministration></UsersAdministration>}
            {currentTabIndex === 1 && <ProvidersAdministration></ProvidersAdministration>}
        </Box >
    )
}