import { Box } from "@mui/material"
import { Tabs, Tab } from "@mui/material"
import { useState } from "react"
import Screenshot from "./Screenshot"
import Actions from "./Actions"

export default function TabularControl({ deviceData }) {
    const udid = deviceData.udid

    const [currentTabIndex, setCurrentTabIndex] = useState(0)

    const handleTabChange = (e, tabIndex) => {
        setCurrentTabIndex(tabIndex)
    }

    return (
        <Box sx={{ width: '100%' }}>
            <Tabs
                value={currentTabIndex}
                onChange={handleTabChange}
                TabIndicatorProps={{ style: { background: "#496612", height: "5px" } }} textColor='white' sx={{ color: "white", fontFamily: "Verdana" }}
            >
                <Tab label="Actions" style={{ textTransform: 'none', fontSize: '16px' }} />
                <Tab label="Logs" style={{ textTransform: 'none', fontSize: '16px' }} />
                <Tab label="Screenshot" style={{ textTransform: 'none', fontSize: '16px' }} />
                <Tab label="Other" style={{ textTransform: 'none', fontSize: '16px' }} />
            </Tabs>
            {currentTabIndex === 2 && <Screenshot udid={udid}></Screenshot>}
            {currentTabIndex === 0 && <Actions deviceData={deviceData}></Actions>}
        </Box >
    )
}