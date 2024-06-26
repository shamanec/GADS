import { useContext, useEffect, useState } from "react";
import { api } from "../../../services/api";
import { Auth } from "../../../contexts/Auth";
import { Box, Button, Grid, MenuItem, Select, Stack, TextField } from "@mui/material";

export default function DeviceList() {
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
        <Box>
            <Grid
                container
                spacing={2}
                style={{
                    marginBottom: '10px'
                }}
            >
                {devices.map((device) => {
                    return (
                        <Grid item>
                            <ExistingDevice
                                deviceData={device}
                                providersData={providers}
                            >
                            </ExistingDevice>
                        </Grid>
                    )
                })
                }
            </Grid>
        </Box>
    )
}

function ExistingDevice({ deviceData, providersData }) {
    const [provider, setProvider] = useState(deviceData.provider)
    const [os, setOS] = useState(deviceData.os)
    const [name, setName] = useState(deviceData.name)
    const [osVersion, setOSVersion] = useState(deviceData.os_version)
    const [screenHeight, setScreenHeight] = useState(deviceData.screen_height)
    const [screenWidth, setScreenWidth] = useState(deviceData.screen_width)
    const udid = deviceData.udid

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

        api.put(url, deviceData)
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
                        padding: '20px'
                    }}
                >
                    <TextField
                        disabled
                        label="UDID"
                        defaultValue={udid}
                    />
                    <TextField
                        required
                        label="Name"
                        defaultValue={name}
                        onChange={(event) => setName(event.target.value)}
                    />
                    <TextField
                        required
                        label="OS Version"
                        defaultValue={osVersion}
                        onChange={(event) => setOSVersion(event.target.value)}
                    />
                    <TextField
                        required
                        label="Screen height"
                        defaultValue={screenHeight}
                        onChange={(event) => setScreenHeight(event.target.value)}
                    />
                    <TextField
                        required
                        label="Screen width"
                        defaultValue={screenWidth}
                        onChange={(event) => setScreenWidth(event.target.value)}
                    />
                    <Select
                        value={os}
                        onChange={(event) => setOS(event.target.value)}
                        style={{ width: '223px', marginTop: "20px" }}
                    >
                        <MenuItem value='android'>Android</MenuItem>
                        <MenuItem value='ios'>iOS</MenuItem>
                    </Select>
                    <Select
                        value={provider}
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
                    >Update device</Button>
                </Stack>
            </form>
        </Box>
    )
}