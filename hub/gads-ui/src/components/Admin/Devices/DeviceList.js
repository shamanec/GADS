import { useContext, useState } from "react"
import { api } from "../../../services/api"
import { Box, Button, FormControl, Grid, MenuItem, Stack, TextField } from "@mui/material"
import { Auth } from "../../../contexts/Auth"

export default function DeviceList({ devices, providers }) {
    const [devicesData, setDevicesData] = useState(devices)
    const [providersData, setProvidersData] = useState(providers)
    const { logout } = useContext(Auth)

    return (
        <Box
            style={{
                marginBottom: '10px',
                height: '80vh',
                overflowY: 'scroll',
                border: '2px solid black',
                borderRadius: '10px',
                boxShadow: 'inset 0 -10px 10px -10px #000000',
                scrollbarWidth: 'none',
                maxWidth: '1700px'
            }}
        >
            <Grid
                container
                spacing={2}
                marginLeft='5px'
                marginTop='5px'
                marginBottom='20px'
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
            })
    }

    function handleDeleteDevice(event) {
        event.preventDefault()

        let url = `/admin/device`

        const deviceData = {
            udid: udid
        }

        api.delete(url, deviceData)
            .catch(e => {
            })
    }

    return (
        <Box
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
                            onChange={(e) => setOS(e.target.value)}
                            select
                            label="Provider"
                            required
                        >
                            {providersData.map((providerName) => {
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
                    >Update device</Button>
                    <Button>Delete device</Button>
                </Stack>
            </form>
        </Box>
    )
}