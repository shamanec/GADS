import { Box, Tab, Tabs } from "@mui/material"
import Info from "./Info"
import { useEffect, useState } from "react"
import Provider from "./Provider/Provider"

export default function Providers({ providers }) {
    const [currentTabIndex, setCurrentTabIndex] = useState(0)
    const [providerInfo, setProviderInfo] = useState(providers[0])

    const handleTabChange = (e, tabIndex) => {
        setCurrentTabIndex(tabIndex)
        console.log('setting')
        console.log(providers[tabIndex])
        setProviderInfo(providers[tabIndex])
    }

    return (
        <Box style={{ margin: '10px' }}>
            <Tabs
                value={currentTabIndex}
                onChange={handleTabChange}
                TabIndicatorProps={{ style: { background: "#496612", height: "5px" } }} textColor='white' sx={{ color: "white", fontFamily: "Verdana" }}
            >
                {providers.map((provider) => {
                    return (
                        <Tab label={provider.nickname} style={{ textTransform: 'none', fontSize: '16px' }} />
                    )
                })
                }
            </Tabs>
            {providerInfo &&
                <Provider isNew={false} info={providerInfo}></Provider>
            }
        </Box>

    )
}