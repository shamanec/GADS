import Tabs from '@mui/material/Tabs';
import Tab from '@mui/material/Tab';
import { useState } from "react";
import { Box } from "@mui/material";
import UsersAdministration from "./UsersAdministration";
import ProvidersAdministration from "./ProvidersAdministration";


export default function AdminDashboard() {
    const [currentTabIndex, setCurrentTabIndex] = useState(0)

    const handleTabChange = (e, tabIndex) => {
        setCurrentTabIndex(tabIndex)
    }

    return (
        <Box id='dashboard-box' style={{ width: '100%', height: '100%' }}>
            <Tabs
                value={currentTabIndex}
                onChange={handleTabChange}
                TabIndicatorProps={{ style: { background: "#496612", height: "5px" } }} textColor='white' sx={{ color: "white", fontFamily: "Verdana" }}
            >
                <Tab label="User administration" style={{ textTransform: 'none', fontSize: '16px' }} />
                <Tab label="Providers administration" style={{ textTransform: 'none', fontSize: '16px' }} />
            </Tabs>
            {currentTabIndex === 0 && <UsersAdministration></UsersAdministration>}
            {currentTabIndex === 1 && <ProvidersAdministration></ProvidersAdministration>}
        </Box >
    )
}