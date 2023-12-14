import { Auth } from "../../contexts/Auth"
import { useContext, useState, useEffect } from "react"
import Tabs from '@mui/material/Tabs';
import Tab from '@mui/material/Tab';
import Provider from "./Provider/Provider";
import axios from "axios";
import Skeleton from '@mui/material/Skeleton';
import SmartphoneIcon from '@mui/icons-material/Smartphone';
import LanIcon from '@mui/icons-material/Lan';
import Tooltip from '@mui/material/Tooltip';
import { ListItem, ListItemIcon, ListItemText } from '@mui/material';
import Box from '@mui/material/Box';
import { Stack } from "@mui/material";


export default function ProvidersAdministration() {
    const [authToken, , logout] = useContext(Auth)
    const [providers, setProviders] = useState([])
    const [currentTabIndex, setCurrentTabIndex] = useState(0)
    const [providerInfo, setProviderInfo] = useState()
    const [isLoading, setIsLoading] = useState(true)

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
        }, 1500);

    }, [])

    return (
        <div style={{ marginTop: '10px' }}>
            {
                isLoading ? (
                    <Skeleton variant="rounded" width='100%' height={60} sx={{ background: '#496612', animationDuration: '1s' }} />
                ) : (
                    <Stack height='800px' spacing={2} width='230px' sx={{ overflowY: 'scroll' }}>
                        {providers.map((provider) => (
                            <Provider info={provider}></Provider>
                        ))}
                    </Stack>
                )
            }
        </div>
    )
}