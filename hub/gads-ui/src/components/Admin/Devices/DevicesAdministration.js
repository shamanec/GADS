import { useContext, useState, useEffect } from "react"
import { api } from "../../../services/api"
import { Box, Button, Dialog, DialogActions, DialogContent, DialogContentText, DialogTitle, FormControl, Grid, MenuItem, Stack, TextField } from "@mui/material"
import { Auth } from "../../../contexts/Auth"

export default function DevicesAdministration() {
    const [devices, setDevices] = useState([])
    const [providers, setProviders] = useState([])
    const { logout } = useContext(Auth)

    function handleGetDeviceData() {
        let url = `/admin/devices`
        // setDevices([])

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
            })
    }

    useEffect(() => {
        handleGetDeviceData()
    }, [])

    return (
        <Stack direction='row' spacing={2} style={{ width: '100%', marginLeft: '10px', marginTop: '10px' }}>
            <Box
                style={{
                    marginBottom: '10px',
                    height: '80vh',
                    overflowY: 'scroll',
                    border: '2px solid black',
                    borderRadius: '10px',
                    boxShadow: 'inset 0 -10px 10px -10px #000000',
                    scrollbarWidth: 'none',
                    marginRight: '10px',
                    width: '100%'
                }}
            >
                <Grid
                    container
                    spacing={2}
                    margin='10px'
                >
                    <Grid item>
                        <NewDevice providers={providers} handleGetDeviceData={handleGetDeviceData}>
                        </NewDevice>
                    </Grid>
                    {devices.map((device) => {
                        return (
                            <Grid item>
                                <ExistingDevice
                                    deviceData={device}
                                    providersData={providers}
                                    handleGetDeviceData={handleGetDeviceData}
                                >
                                </ExistingDevice>
                            </Grid>
                        )
                    })
                    }
                </Grid>
            </Box>
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
                height: '630px',
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
                        autoComplete="off"
                        onChange={(event) => setUdid(event.target.value)}
                    />
                    <TextField
                        required
                        label="Name"
                        value={name}
                        autoComplete="off"
                        onChange={(event) => setName(event.target.value)}
                    />
                    <TextField
                        required
                        label="OS Version"
                        value={osVersion}
                        autoComplete="off"
                        onChange={(event) => setOSVersion(event.target.value)}
                    />
                    <TextField
                        required
                        label="Screen height"
                        value={screenHeight}
                        autoComplete="off"
                        onChange={(event) => setScreenHeight(event.target.value)}
                    />
                    <TextField
                        required
                        label="Screen width"
                        value={screenWidth}
                        autoComplete="off"
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
                        style={{
                            backgroundColor: '#2f3b26',
                            color: '#f4e6cd'
                        }}
                    >Add device</Button>
                </Stack>
            </form>
        </Box >
    )
}

function ExistingDevice({ deviceData, providersData, handleGetDeviceData }) {
    const [provider, setProvider] = useState(deviceData.provider)
    const [os, setOS] = useState(deviceData.os)
    const [name, setName] = useState(deviceData.name)
    const [osVersion, setOSVersion] = useState(deviceData.os_version)
    const [screenHeight, setScreenHeight] = useState(deviceData.screen_height)
    const [screenWidth, setScreenWidth] = useState(deviceData.screen_width)
    const udid = deviceData.udid

    useEffect(() => {
        setProvider(deviceData.provider)
        setOS(deviceData.os)
        setName(deviceData.name)
        setOSVersion(deviceData.os_version)
        setScreenHeight(deviceData.screen_height)
        setScreenWidth(deviceData.screen_width)
    }, [deviceData])

    function handleUpdateDevice(event) {
        event.preventDefault()

        let url = `/admin/device`

        const reqData = {
            udid: udid,
            name: name,
            os_version: osVersion,
            provider: provider,
            screen_height: screenHeight,
            screen_width: screenWidth,
            os: os
        }

        api.put(url, reqData)
            .catch(e => {

            })
            .finally(() => {
                handleGetDeviceData()
            })
    }

    function handleDeleteDevice(event) {
        event.preventDefault()

        let url = `/admin/device/${udid}`

        api.delete(url)
            .catch(e => {
            })
            .finally(() => {
                handleGetDeviceData()
                setOpenAlert(false)
            })
    }

    const [openAlert, setOpenAlert] = useState(false)

    return (
        <Box
            style={{
                border: '1px solid black',
                width: '400px',
                minWidth: '400px',
                maxWidth: '400px',
                height: '630px',
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
                        autoComplete="off"
                        onChange={(event) => setName(event.target.value)}
                    />
                    <TextField
                        required
                        label="OS Version"
                        defaultValue={osVersion}
                        autoComplete="off"
                        onChange={(event) => setOSVersion(event.target.value)}
                    />
                    <TextField
                        required
                        label="Screen height"
                        defaultValue={screenHeight}
                        autoComplete="off"
                        onChange={(event) => setScreenHeight(event.target.value)}
                    />
                    <TextField
                        required
                        label="Screen width"
                        defaultValue={screenWidth}
                        autoComplete="off"
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
                        style={{
                            backgroundColor: '#2f3b26',
                            color: '#f4e6cd'
                        }}
                    >Update device</Button>
                    <Button
                        onClick={() => setOpenAlert(true)}
                        style={{
                            backgroundColor: 'orange',
                            color: '#2f3b26'
                        }}
                    >Delete device</Button>
                    <Dialog
                        open={openAlert}
                        onClose={() => setOpenAlert(false)}
                        aria-labelledby="alert-dialog-title"
                        aria-describedby="alert-dialog-description"
                    >
                        <DialogTitle id="alert-dialog-title">
                            {"Delete device from DB?"}
                        </DialogTitle>
                        <DialogContent>
                            <DialogContentText id="alert-dialog-description">
                                Device with UDID `{udid}`, assigned to provider `{provider}`. Confirm deletion?
                            </DialogContentText>
                        </DialogContent>
                        <DialogActions>
                            <Button onClick={() => setOpenAlert(false)}>Cancel</Button>
                            <Button onClick={handleDeleteDevice} autoFocus>
                                Confirm
                            </Button>
                        </DialogActions>
                    </Dialog>
                </Stack>
            </form>
        </Box>
    )
}