import { Box } from "@mui/material"
import { Tabs, Tab } from "@mui/material"
import { useState } from "react"
import Screenshot from "./Screenshot/Screenshot"
import Actions from "./Actions/Actions"

export default function TabularControl({ deviceData }) {
    const udid = deviceData.Device.udid

    const [currentTabIndex, setCurrentTabIndex] = useState(0)

    const handleTabChange = (e, tabIndex) => {
        setCurrentTabIndex(tabIndex)
    }

    return (
        <Box sx={{ width: '100%' }}>
            <Tabs
                value={currentTabIndex}
                onChange={handleTabChange}
                TabIndicatorProps={{ style: { background: '#496612', height: '5px' } }} textColor='white' sx={{ color: 'white', fontFamily: 'Verdana' }}
            >
                <Tab className='control-tabs' label='Actions' />
                <Tab className='control-tabs' label='Screenshot' />
            </Tabs>
            {currentTabIndex === 1 && <Screenshot udid={udid} />}
            {currentTabIndex === 0 && <Actions deviceData={deviceData} />}
        </Box >
    )
}