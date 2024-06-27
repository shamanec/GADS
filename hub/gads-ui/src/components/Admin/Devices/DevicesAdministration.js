import DeviceList from "./DeviceList";
import { useContext, useState, useEffect } from "react";
import { api } from "../../../services/api";
import { Box, Button, Divider, FormControl, MenuItem, Stack, TextField } from "@mui/material";
import { Auth } from "../../../contexts/Auth";

export default function DevicesAdministration() {
    const [devices, setDevices] = useState([])
    const [providers, setProviders] = useState([])
    const { logout } = useContext(Auth)

    function handleGetDeviceData() {
        let url = `/admin/devices`

        api.get(url)
            .then(response => {
                console.log('lol')
                console.log(response.data.devices)
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
    }

    useEffect(() => {
        handleGetDeviceData()
    }, [])

    return (
        <Stack direction='row' spacing={2} style={{ marginLeft: '10px', marginTop: '10px' }}>
            <NewDevice providers={providers} handleGetDeviceData={handleGetDeviceData}>
            </NewDevice>
            <Divider orientation="vertical" flexItem style={{ borderRightWidth: '5px', height: '80vh' }}>
            </Divider>
            <DeviceList devices={devices} providers={providers}>
            </DeviceList>
        </Stack>
    )
}

function NewDevice({ providers, handleGetDeviceData }) {
    const [udid, setUdid] = useState('')
    const [provider, setProvider] = useState('')
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
            .finally(() => {
                setUdid('')
                setProvider('')
                setOS('')
                setName('')
                setOSVersion('')
                setScreenHeight('')
                setScreenWidth('')
                handleGetDeviceData()
            })
    }

    return (
        <Box
            id='some-box'
            style={{
                border: '1px solid black',
                width: '400px',
                minWidth: '400px',
                maxWidth: '400px',
                height: '600px',
                borderRadius: '5px',
                backgroundColor: '#9ba984'
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
                        value={udid}
                        onChange={(event) => setUdid(event.target.value)}
                    />
                    <TextField
                        required
                        label="Name"
                        value={name}
                        onChange={(event) => setName(event.target.value)}
                    />
                    <TextField
                        required
                        label="OS Version"
                        value={osVersion}
                        onChange={(event) => setOSVersion(event.target.value)}
                    />
                    <TextField
                        required
                        label="Screen height"
                        value={screenHeight}
                        onChange={(event) => setScreenHeight(event.target.value)}
                    />
                    <TextField
                        required
                        label="Screen width"
                        value={screenWidth}
                        onChange={(event) => setScreenWidth(event.target.value)}
                    />
                    <FormControl fullWidth variant="outlined" required>
                        <TextField
                            style={{ width: "100%" }}
                            variant="outlined"
                            value={os}
                            onChange={(e) => setOS(e.target.value)}
                            select
                            label="Device OS"
                            required
                        >
                            <MenuItem value='android'>Android</MenuItem>
                            <MenuItem value='ios'>iOS</MenuItem>
                        </TextField>
                    </FormControl>
                    <FormControl fullWidth variant="outlined" required>
                        <TextField
                            style={{ width: "100%" }}
                            variant="outlined"
                            value={provider}
                            onChange={(e) => setProvider(e.target.value)}
                            select
                            label="Provider"
                            required
                        >
                            {providers.map((providerName) => {
                                return (
                                    <MenuItem id={providerName} value={providerName}>{providerName}</MenuItem>
                                )
                            })
                            }
                        </TextField>
                    </FormControl>
                    <Button
                        variant="contained"
                        type="submit"
                    >Add device</Button>
                </Stack>
            </form>
        </Box >
    )
}