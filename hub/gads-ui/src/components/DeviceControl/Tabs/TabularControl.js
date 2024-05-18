import { Box } from "@mui/material"
import { Tabs, Tab } from "@mui/material"
import { useState } from "react"
import Screenshot from "./Screenshot/Screenshot"
import Actions from "./Actions/Actions"
import AppiumLogsTable from "./Logs/AppiumLogsTable";

export default function TabularControl({ deviceData }) {
    const udid = deviceData.udid

    const [currentTabIndex, setCurrentTabIndex] = useState(0)

    const handleTabChange = (e, tabIndex) => {
        setCurrentTabIndex(tabIndex)
    }

    return (
        <Box
            sx={{
                width: '100%'
            }}
        >
            <Tabs
                value={currentTabIndex}
                onChange={handleTabChange}
                TabIndicatorProps={{
                    style: {
                        background: '#78866B',
                        height: '5px'
                    }
                }}
                textColor='#78866B'
                sx={{
                    color: '#78866B',
                    fontFamily: 'Verdana'
                }}
            >
                <Tab
                    className='control-tabs'
                    label='Actions'
                    style={{ fontWeight: "bold"}}
                />
                <Tab
                    className='control-tabs'
                    label='Screenshot'
                    style={{ fontWeight: "bold"}}
                />
                <Tab
                    className='control-tabs'
                    label='Appium Logs'
                    style={{ fontWeight: "bold"}}
                />
            </Tabs>
            {currentTabIndex === 1 && <Screenshot udid={udid} />}
            {currentTabIndex === 0 && <Actions deviceData={deviceData} />}
            {currentTabIndex === 2 && <AppiumLogsTable udid={udid} />}
        </Box >
    )
}