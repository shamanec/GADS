import DeviceList from "./DeviceList";
import { useContext, useState, useEffect } from "react";
import { api } from "../../../services/api";
import { Box, Button, Divider, MenuItem, Select, Stack, TextField } from "@mui/material";
import { Auth } from "../../../contexts/Auth";

export default function DevicesAdministration() {
    const [devices, setDevices] = useState([])
    const [providers, setProviders] = useState([])
    const { logout } = useContext(Auth)

    useEffect(() => {
        let url = `/admin/devices`

        api.get(url)
            .then(response => {
                setDevices(response.data.devices)
                setProviders(response.data.providers)
            })
            .catch(error => {
                if (error.response) {
                    if (error.response.status === 401) {
                        logout()
                    }
                }
            });
    }, [])

    return (
        <Stack direction='row' spacing={2} height='80vh' style={{ marginLeft: '10px', marginTop: '10px' }}>
            <NewDevice providersData={providers}></NewDevice>
            <Divider orientation="vertical" flexItem style={{ borderRightWidth: '5px' }}></Divider>
            <DeviceList>
            </DeviceList>
        </Stack>
    )
}

function NewDevice({ providersData }) {
    const [udid, setUdid] = useState('')
    const [provider, setProvider] = useState(providersData[0])
    const [os, setOS] = useState('')
    const [name, setName] = useState('')
    const [osVersion, setOSVersion] = useState('')
    const [screenHeight, setScreenHeight] = useState('')
    const [screenWidth, setScreenWidth] = useState('')

    function handleUpdateDevice(event) {
        event.preventDefault()

        let url = `/admin/device`

        const deviceData = {
            udid: udid,
            name: name,
            os_version: osVersion,
            provider: provider,
            screen_height: screenHeight,
            screen_width: screenWidth,
            os: os
        }

        api.post(url, deviceData)
            .catch(e => {
                console.log('wtf')
                console.log(e)
            })
    }

    return (
        <Box
            style={{
                border: '1px solid black',
                width: '400px',
                borderRadius: '5px'
            }}
        >
            <form onSubmit={handleUpdateDevice}>
                <Stack
                    spacing={2}
                    style={{
                        padding: '10px'
                    }}
                >
                    <TextField
                        required
                        label="UDID"
                        onChange={(event) => setUdid(event.target.value)}
                    />
                    <TextField
                        required
                        label="Name"
                        onChange={(event) => setName(event.target.value)}
                    />
                    <TextField
                        required
                        label="OS Version"
                        onChange={(event) => setOSVersion(event.target.value)}
                    />
                    <TextField
                        required
                        label="Screen height"
                        onChange={(event) => setScreenHeight(event.target.value)}
                    />
                    <TextField
                        required
                        label="Screen width"
                        onChange={(event) => setScreenWidth(event.target.value)}
                    />
                    <Select
                        value='android'
                        onChange={(event) => setOS(event.target.value)}
                        style={{ width: '223px', marginTop: "20px" }}
                    >
                        <MenuItem value='android'>Android</MenuItem>
                        <MenuItem value='ios'>iOS</MenuItem>
                    </Select>
                    <Select
                        value={providersData[0]}
                        onChange={(event) => setProvider(event.target.value)}
                        style={{ width: '223px', marginTop: "20px" }}
                    >
                        {providersData.map((providerName) => {
                            return (
                                <MenuItem id={providerName} value={providerName}>{providerName}</MenuItem>
                            )
                        })
                        }
                    </Select>
                    <Button
                        variant="contained"
                        type="submit"
                    >Add device</Button>
                </Stack>
            </form>
        </Box>
    )
}