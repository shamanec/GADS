import { Box, Tab, Tabs } from "@mui/material"
import { useState } from "react"
import Provider from "./Provider/Provider"

export default function Providers({ providers }) {
    const [currentTabIndex, setCurrentTabIndex] = useState(0)

    const handleTabChange = (e, tabIndex) => {
        setCurrentTabIndex(tabIndex)
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
            <Provider isNew={false} info={providers[currentTabIndex]}></Provider>
        </Box>

    )
}