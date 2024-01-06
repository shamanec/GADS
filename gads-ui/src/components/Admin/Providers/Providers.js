import { Box, Tab, Tabs } from "@mui/material"
import { useState } from "react"
import Provider from "./Provider/Provider"
import ProviderConfig from "./Provider/ProviderConfig"

export default function Providers({ providers, setProviders }) {
    const [currentTabIndex, setCurrentTabIndex] = useState(0)

    const handleTabChange = (e, tabIndex) => {
        setCurrentTabIndex(tabIndex)
    }

    return (
        <Box style={{ margin: '10px', width: '100%' }}>
            <Tabs
                value={currentTabIndex}
                onChange={handleTabChange}
                TabIndicatorProps={{ style: { background: "#496612", height: "5px" } }} textColor='white' sx={{ color: "white", fontFamily: "Verdana" }}
            >
                <Tab label='New provider' style={{ textTransform: 'none', fontSize: '16px' }}></Tab>
                {providers.map((provider) => {
                    return (
                        <Tab label={provider.nickname} style={{ textTransform: 'none', fontSize: '16px' }} />
                    )
                })
                }
            </Tabs>
            {currentTabIndex === 0 && <ProviderConfig isNew={true} setProviders={setProviders} ></ProviderConfig>}
            {currentTabIndex !== 0 && <Provider isNew={false} info={providers[currentTabIndex - 1]}></Provider>}
        </Box >

    )
}