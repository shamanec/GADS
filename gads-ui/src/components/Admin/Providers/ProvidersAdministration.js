import { Auth } from "../../../contexts/Auth"
import { useContext, useState, useEffect } from "react"
import ListProvider from "./ListProvider";
import axios from "axios";
import Skeleton from '@mui/material/Skeleton';
import { Box, Button, Stack } from "@mui/material";
import ProviderConfig from "./Provider/ProviderConfig";
import Provider from "./Provider/Provider";


export default function ProvidersAdministration() {
    const [authToken, , , , logout] = useContext(Auth)
    const [providers, setProviders] = useState([])
    const [isLoading, setIsLoading] = useState(true)
    const [providerInfo, setProviderInfo] = useState({})
    const [showAddProvider, setShowAddProvider] = useState(false)
    const [showProvider, setShowProvider] = useState(false)

    function handleAddProvider() {
        setShowProvider(false)
        setShowAddProvider(true)
    }

    function handleShowProvider() {
        setShowAddProvider(false)
        setShowProvider(true)
    }

    function getProviderInfo(name) {
        for (let i = 0; i < providers.length; i++) {
            if (providers[i].nickname === name) {
                return providers[i]
            }
        }
        return null
    }

    function handleSelectProvider(name) {
        setProviderInfo(getProviderInfo(name))
        handleShowProvider()
    }

    let url = `/admin/providers`

    useEffect(() => {
        axios.get(url, {
            headers: {
                'X-Auth-Token': authToken
            }
        })
            .then((response) => {
                setProviders(response.data)
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
        <Box height='100%'>
            <Stack direction='row' height='80%'>

                <Stack spacing={2} width='230px' height='80%' style={{ overflowY: 'scroll', marginTop: '10px' }}>
                    <Button onClick={handleAddProvider} style={{ height: '50px' }}>Add provider</Button>
                    {
                        isLoading ? (
                            <Skeleton variant="rounded" style={{ marginLeft: '10px', background: '#496612', animationDuration: '1s', height: '100%' }} />
                        ) : (
                            <>
                                {providers.map((provider) => (
                                    <ListProvider handleSelectProvider={handleSelectProvider} info={provider}></ListProvider>
                                ))}
                            </>
                        )
                    }
                </Stack>
                {showAddProvider &&
                    <ProviderConfig isNew={true}></ProviderConfig>
                }
                {showProvider &&
                    <Provider isNew={false} data={providerInfo}></Provider>
                }
            </Stack>

        </Box>

    )
}