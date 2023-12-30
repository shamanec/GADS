import { Auth } from "../../../contexts/Auth"
import { useContext, useState, useEffect } from "react"
import ListProvider from "./ListProvider";
import axios from "axios";
import Skeleton from '@mui/material/Skeleton';
import { Box, Button, Stack } from "@mui/material";
import AddProvider from "./AddProvider/AddProvider";


export default function ProvidersAdministration() {
    const [authToken, , , , logout] = useContext(Auth)
    const [providers, setProviders] = useState([])
    const [isLoading, setIsLoading] = useState(true)
    const [providerInfo, setProviderInfo] = useState({})
    const [showAddProvider, setShowAddProvider] = useState(false)

    function handleAddProvider() {
        setShowAddProvider(true)
    }

    function handleSelectProvider(name) {
        console.log(name)
        let url = `/provider/${name}/info`
        axios.get(url, {
            headers: {
                'X-Auth-Token': authToken
            }
        }).then((response) => {
            setProviderInfo(response.data)
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
                    <AddProvider></AddProvider>
                }
            </Stack>

        </Box>

    )
}