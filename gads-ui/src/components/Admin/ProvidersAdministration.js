import { Auth } from "../../contexts/Auth"
import { useContext, useState, useEffect } from "react"
import Tabs from '@mui/material/Tabs';
import Tab from '@mui/material/Tab';
import Provider from "./Provider/Provider";
import axios from "axios";
import Skeleton from '@mui/material/Skeleton';


export default function ProvidersAdministration() {
    const [authToken, , logout] = useContext(Auth)
    const [providers, setProviders] = useState([])
    const [currentTabIndex, setCurrentTabIndex] = useState(0)
    const [providerInfo, setProviderInfo] = useState()
    const [isLoading, setIsLoading] = useState(true)

    const handleTabChange = (e, tabIndex) => {
        setCurrentTabIndex(tabIndex)
        setProviderInfo(providers[tabIndex])
    }

    let url = `http://${process.env.REACT_APP_GADS_BACKEND_HOST}/admin/providers`

    useEffect(() => {
        axios.get(url, {
            headers: {
                'X-Auth-Token': authToken
            }
        })
            .then((response) => {
                setProviders(response.data)
                setProviderInfo(response.data[0])
            })
            .catch(error => {
                if (error.response) {
                    if (error.response.status === 401) {
                        logout()
                        return
                    }
                }
                console.log('Failed getting providers data' + error)
                return
            });

        setInterval(() => {
            setIsLoading(false)
        }, 2000);

    }, [])

    return (
        <div style={{ marginTop: '10px' }}>
            {
                isLoading ? (
                    <Skeleton variant="rounded" width='100%' height={60} sx={{ background: '#496612', animationDuration: '1s' }} />
                ) : (
                    <div>
                        <Tabs
                            value={currentTabIndex}
                            onChange={handleTabChange}
                            TabIndicatorProps={{ style: { background: "#496612", height: "5px" } }} textColor='white' sx={{ color: "white", fontFamily: "Verdana" }}
                        >
                            {providers.map((provider) => (
                                <Tab label={provider.name} style={{ textTransform: 'none', fontSize: '16px' }} />
                            ))}

                        </Tabs>
                        <Provider info={providerInfo}></Provider>
                    </div>

                )
            }
        </div>
    )
}