import { Auth } from "../../contexts/Auth"
import { useContext, useState, useEffect } from "react"
import Tabs from '@mui/material/Tabs';
import Tab from '@mui/material/Tab';
import Provider from "./Provider/Provider";


export default function ProvidersAdministration() {
    const [authToken, , logout] = useContext(Auth)
    const [providers, setProviders] = useState([])
    const [currentTabIndex, setCurrentTabIndex] = useState(0)
    const [providerInfo, setProviderInfo] = useState()

    const handleTabChange = (e, tabIndex) => {
        setCurrentTabIndex(tabIndex)
        setProviderInfo(providers[tabIndex])
    }

    let url = `http://${process.env.REACT_APP_GADS_BACKEND_HOST}/admin/providers`

    useEffect(() => {
        fetch(url, {
            method: 'GET',
            headers: {
                'X-Auth-Token': authToken
            }
        })
            .then(response => {
                if (response.status === 401) {
                    logout()
                    return
                } else if (response.status != 200) {
                    setProviders([])
                } else {
                    return response.json()
                }
            })
            .then(json => {
                setProviders(json)
                setProviderInfo(json[0])
            })
            .catch()
    }, [])

    console.log(providerInfo)

    return (
        <Tabs
            value={currentTabIndex}
            onChange={handleTabChange}
            TabIndicatorProps={{ style: { background: "#496612", height: "5px" } }} textColor='white' sx={{ color: "white", fontFamily: "Verdana" }}
        >
            {providers.map((provider) => (
                <Tab label={provider.name} style={{ textTransform: 'none', fontSize: '16px' }} />
            ))}
            <Provider info={providerInfo}></Provider>
        </Tabs>
    )
}