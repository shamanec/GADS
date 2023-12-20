import { Box } from "@mui/material"
import { Tabs, Tab } from "@mui/material"
import { useState } from "react"
import Screenshot from "./Screenshot/Screenshot"
import Actions from "./Actions/Actions"

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
                TabIndicatorProps={{ style: { background: '#496612', height: '5px' } }} textColor='white' sx={{ color: 'white', fontFamily: 'Verdana' }}
            >
                <Tab className='control-tabs' label='Actions' />
                <Tab className='control-tabs' label='Logs' />
                <Tab className='control-tabs' label='Screenshot' />
                <Tab className='control-tabs' label='Other' />
            </Tabs>
            {currentTabIndex === 2 && <Screenshot udid={udid} />}
            {currentTabIndex === 0 && <Actions deviceData={deviceData} />}
        </Box >
    )
}