import { useContext, useState, useEffect } from 'react'
import { api } from '../../../services/api'
import { Box, Button, Dialog, DialogActions, DialogContent, DialogContentText, DialogTitle, FormControl, Grid, MenuItem, Stack, TextField, Tooltip } from '@mui/material'
import { Auth } from '../../../contexts/Auth'
import CircularProgress from '@mui/material/CircularProgress'
import CheckIcon from '@mui/icons-material/Check'
import CloseIcon from '@mui/icons-material/Close'
import { useDialog } from '../../../contexts/DialogContext'

export default function DevicesAdministration() {
    const [devices, setDevices] = useState([])
    const [providers, setProviders] = useState([])
    const { logout } = useContext(Auth)

    function handleGetDeviceData() {
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
    const [usage, setUsage] = useState('enabled')
    const [type, setType] = useState('real')

    const [loading, setLoading] = useState(false)
    const [addDeviceStatus, setAddDeviceStatus] = useState(null)

    function handleAddDevice(event) {
        setLoading(true)
        setAddDeviceStatus(null)
        event.preventDefault()

        let url = `/admin/device`

        const deviceData = {
            udid: udid,
            name: name,
            os_version: osVersion,
            provider: provider,
            screen_height: screenHeight,
            screen_width: screenWidth,
            os: os,
            usage: usage,
            device_type: type
        }

        api.post(url, deviceData)
            .then(() => {
                setAddDeviceStatus('success')
                setUdid('')
                setProvider('')
                setOS('')
                setName('')
                setOSVersion('')
                setScreenHeight('')
                setScreenWidth('')
                setUsage('enabled')
            })
            .catch(() => {
                setAddDeviceStatus('error')
            })
            .finally(() => {
                setTimeout(() => {
                    setLoading(false)
                    handleGetDeviceData()
                    setTimeout(() => {
                        setAddDeviceStatus(null)
                    }, 2000)
                }, 1000)
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
                height: '830px',
                borderRadius: '5px',
                backgroundColor: '#9ba984'
            }}
        >
            <form onSubmit={handleAddDevice}>
                <Stack
                    spacing={2}
                    style={{
                        padding: '20px'
                    }}
                >
                    <Tooltip
                        title='Operating system of the device'
                        arrow
                        placement='top'
                    >
                        <FormControl fullWidth variant='outlined' required>
                            <TextField
                                style={{ width: '100%' }}
                                variant='outlined'
                                value={os}
                                onChange={(e) => setOS(e.target.value)}
                                select
                                label='Device OS'
                                required
                            >
                                <MenuItem value='android'>Android</MenuItem>
                                <MenuItem value='ios'>iOS</MenuItem>
                            </TextField>
                        </FormControl>
                    </Tooltip>
                    <Tooltip
                        title='Type of device - real device or simulator/emulator'
                        arrow
                        placement='top'
                    >
                        <FormControl fullWidth variant='outlined' required>
                            <TextField
                                style={{ width: '100%' }}
                                variant='outlined'
                                value={type}
                                onChange={(e) => setType(e.target.value)}
                                select
                                label='Device type'
                                required
                            >
                                <MenuItem value='real'>Real device</MenuItem>
                                <MenuItem disabled value='emulator'>Emulator/Simulator - TODO</MenuItem>
                            </TextField>
                        </FormControl>
                    </Tooltip>
                    <Tooltip
                        title={<div>Unique device identifier<br />Use `adb devices` to get Android device UDID<br />Use `ios list` to get iOS device UDID with `go-ios`</div>}
                        arrow
                        placement='top'
                    >
                        <TextField
                            required
                            label='UDID'
                            value={udid}
                            autoComplete='off'
                            onChange={(event) => setUdid(event.target.value)}
                        />
                    </Tooltip>
                    <Tooltip
                        title='Unique name for the device, e.g. iPhone SE(2nd gen)'
                        arrow
                        placement='top'
                    >
                        <TextField
                            required
                            label='Name'
                            value={name}
                            autoComplete='off'
                            onChange={(event) => setName(event.target.value)}
                        />
                    </Tooltip>
                    <Tooltip
                        title='Device OS version, major or exact e.g 17 or 17.5.1'
                        arrow
                        placement='top'
                    >
                        <TextField
                            required
                            label='OS Version'
                            value={osVersion}
                            autoComplete='off'
                            onChange={(event) => setOSVersion(event.target.value)}
                        />
                    </Tooltip>
                    <Tooltip
                        title={<div>Device screen width - Optional. Set manually if you find a problem with the automatic values.<br />For Android - go to `https://whatismyandroidversion.com` and use the displayed `Screen size`, not `Viewport size`<br />For iOS - you can get it on https://whatismyviewport.com (ScreenSize: at the bottom)</div>}
                        arrow
                        placement='top'
                    >
                        <TextField
                            label='Screen width'
                            value={screenWidth}
                            autoComplete='off'
                            onChange={(event) => setScreenWidth(event.target.value)}
                        />
                    </Tooltip>
                    <Tooltip
                        title={<div>Device screen height - Optional. Set manually if you find a problem with the automatic values.<br />For Android - go to `https://whatismyandroidversion.com` and use the displayed `Screen size`, not `Viewport size`<br />For iOS - you can get it on https://whatismyviewport.com (ScreenSize: at the bottom)</div>}
                        arrow
                        placement='top'
                    >
                        <TextField
                            label='Screen height'
                            value={screenHeight}
                            autoComplete='off'
                            onChange={(event) => setScreenHeight(event.target.value)}
                        />
                    </Tooltip>
                    <Tooltip
                        title={<div>Intended usage of the device <br />Enabled: Can be used for automation and remote control <br />Automation: Can be used only as automation target <br />Remote control: Can be used only for remote control testing <br />Disabled: Device will not be provided</div>}
                        arrow
                        placement='top'
                    >
                        <FormControl fullWidth variant='outlined' required>
                            <TextField
                                style={{ width: '100%' }}
                                variant='outlined'
                                value={usage}
                                onChange={(e) => setUsage(e.target.value)}
                                select
                                label='Device usage'
                                required
                            >
                                <MenuItem value='enabled'>Enabled</MenuItem>
                                <MenuItem value='automation'>Automation</MenuItem>
                                <MenuItem value='control'>Remote control</MenuItem>
                                <MenuItem value='disabled'>Disabled</MenuItem>
                            </TextField>
                        </FormControl>
                    </Tooltip>
                    <Tooltip
                        title='The nickname of the provider to which the device is assigned'
                        arrow
                        placement='top'
                    >
                        <FormControl fullWidth variant='outlined' required>
                            <TextField
                                style={{ width: '100%' }}
                                variant='outlined'
                                value={provider}
                                onChange={(e) => setProvider(e.target.value)}
                                select
                                label='Provider'
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
                    </Tooltip>
                    <Button
                        variant='contained'
                        type='submit'
                        style={{
                            backgroundColor: '#2f3b26',
                            color: '#f4e6cd',
                            fontWeight: 'bold',
                            boxShadow: 'none',
                            height: '40px'
                        }}
                        disabled={loading || addDeviceStatus === 'success' || addDeviceStatus === 'error'}
                    >
                        {loading ? (
                            <CircularProgress size={25} style={{ color: '#f4e6cd' }} />
                        ) : addDeviceStatus === 'success' ? (
                            <CheckIcon size={25} style={{ color: '#f4e6cd', stroke: '#f4e6cd', strokeWidth: 2 }} />
                        ) : addDeviceStatus === 'error' ? (
                            <CloseIcon size={25} style={{ color: 'red', stroke: 'red', strokeWidth: 2 }} />
                        ) : (
                            'Add device'
                        )}
                    </Button>
                    <div>All updates to existing devices require respective provider restart</div>
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
    const [usage, setUsage] = useState(deviceData.usage)
    const [type, setType] = useState(deviceData.device_type)
    const udid = deviceData.udid

    const [loading, setLoading] = useState(false)
    const [reprovisionLoading, setReprovisionLoading] = useState(false)
    const [updateDeviceStatus, setUpdateDeviceStatus] = useState(null)
    const [reprovisionDeviceStatus, setReprovisionDeviceStatus] = useState(null)

    useEffect(() => {
        setProvider(deviceData.provider)
        setOS(deviceData.os)
        setName(deviceData.name)
        setOSVersion(deviceData.os_version)
        setScreenHeight(deviceData.screen_height)
        setScreenWidth(deviceData.screen_width)
    }, [deviceData])

    function handleUpdateDevice(event) {
        setLoading(true)
        setUpdateDeviceStatus(null)
        event.preventDefault()

        let url = `/admin/device`

        const reqData = {
            udid: udid,
            name: name,
            os_version: osVersion,
            provider: provider,
            screen_height: screenHeight,
            screen_width: screenWidth,
            os: os,
            usage: usage,
            device_type: type
        }

        api.put(url, reqData)
            .then(() => {
                setUpdateDeviceStatus('success')
            })
            .catch(() => {
                setUpdateDeviceStatus('error')
            })
            .finally(() => {
                setTimeout(() => {
                    setLoading(false)
                    handleGetDeviceData()
                    setTimeout(() => {
                        setUpdateDeviceStatus(null)
                    }, 2000)
                }, 1000)
            })
    }

    function handleReprovisionDevice(event) {
        setReprovisionLoading(true)
        setReprovisionDeviceStatus(null)
        event.preventDefault()

        let url = `/device/${udid}/reset`

        api.post(url)
            .then(() => {
                setReprovisionDeviceStatus('success')
            })
            .catch(() => {
                setReprovisionDeviceStatus('error')
            })
            .finally(() => {
                setTimeout(() => {
                    setReprovisionLoading(false)
                    setTimeout(() => {
                        setReprovisionDeviceStatus(null)
                    }, 2000)
                }, 1000)
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
            })
    }

    const { showDialog, hideDialog } = useDialog()
    const showDeleteDeviceAlert = (event) => {

        showDialog('deleteDeviceAlert', {
            title: 'Delete device from DB?',
            content: `Device with UDID '${udid}', assigned to provider '${provider}'.`,
            actions: [
                { label: 'Cancel', onClick: () => hideDialog() },
                { label: 'Confirm', onClick: () => handleDeleteDevice(event) }
            ],
            isCloseable: false
        })
    }

    return (
        <Box
            style={{
                border: '1px solid black',
                width: '400px',
                minWidth: '400px',
                maxWidth: '400px',
                height: '830px',
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
                    <Tooltip
                        title='Operating system of the device'
                        arrow
                        placement='top'
                    >
                        <FormControl fullWidth variant='outlined' required>
                            <TextField
                                disabled
                                style={{ width: '100%' }}
                                variant='outlined'
                                value={os}
                                onChange={(e) => setOS(e.target.value)}
                                select
                                label='Device OS'
                                required
                            >
                                <MenuItem value='android'>Android</MenuItem>
                                <MenuItem value='ios'>iOS</MenuItem>
                            </TextField>
                        </FormControl>
                    </Tooltip>
                    <Tooltip
                        title='Type of device - real device or simulator/emulator'
                        arrow
                        placement='top'
                    >
                        <FormControl fullWidth variant='outlined' required>
                            <TextField
                                style={{ width: '100%' }}
                                variant='outlined'
                                value={type}
                                select
                                label='Device type'
                                required
                                disabled
                            >
                                <MenuItem value='real'>Real device</MenuItem>
                                <MenuItem value='emulator'>Emulator/Simulator</MenuItem>
                            </TextField>
                        </FormControl>
                    </Tooltip>
                    <Tooltip
                        title={udid}
                        arrow
                        placement='top'
                    >
                        <TextField
                            disabled
                            label='UDID'
                            defaultValue={udid}
                        />
                    </Tooltip>
                    <Tooltip
                        title='Unique name for the device, e.g. iPhone SE(2nd gen)'
                        arrow
                        placement='top'
                    >
                        <TextField
                            required
                            label='Name'
                            defaultValue={name}
                            autoComplete='off'
                            onChange={(event) => setName(event.target.value)}
                        />
                    </Tooltip>
                    <Tooltip
                        title='Device OS version, major or exact e.g 17 or 17.5.1'
                        arrow
                        placement='top'
                    >
                        <TextField
                            required
                            label='OS Version'
                            defaultValue={osVersion}
                            autoComplete='off'
                            onChange={(event) => setOSVersion(event.target.value)}
                        />
                    </Tooltip>
                    <Tooltip
                        title={<div>Device screen width<br />For Android - go to `https://whatismyandroidversion.com` and use the displayed `Screen size`, not `Viewport size`<br />For iOS - you can get it on https://whatismyviewport.com (ScreenSize: at the bottom)</div>}
                        arrow
                        placement='top'
                    >
                        <TextField
                            label='Screen width'
                            defaultValue={screenWidth}
                            autoComplete='off'
                            onChange={(event) => setScreenWidth(event.target.value)}
                        />
                    </Tooltip>
                    <Tooltip
                        title={<div>Device screen height<br />For Android - go to `https://whatismyandroidversion.com` and use the displayed `Screen size`, not `Viewport size`<br />For iOS - you can get it on https://whatismyviewport.com (ScreenSize: at the bottom)</div>}
                        arrow
                        placement='top'
                    >
                        <TextField
                            label='Screen height'
                            defaultValue={screenHeight}
                            autoComplete='off'
                            onChange={(event) => setScreenHeight(event.target.value)}
                        />
                    </Tooltip>
                    <Tooltip
                        title={<div>Intended usage of the device <br />Enabled: Can be used for automation and remote control <br />Automation: Can be used only as automation target <br />Remote control: Can be used only for remote control testing <br />Disabled: Device will not be provided</div>}
                        arrow
                        placement='top'
                    >
                        <FormControl fullWidth variant='outlined' required>
                            <TextField
                                style={{ width: '100%' }}
                                variant='outlined'
                                value={usage}
                                onChange={(e) => setUsage(e.target.value)}
                                select
                                label='Device usage'
                                required
                            >
                                <MenuItem value='enabled'>Enabled</MenuItem>
                                <MenuItem value='automation'>Automation</MenuItem>
                                <MenuItem value='control'>Remote control</MenuItem>
                                <MenuItem value='disabled'>Disabled</MenuItem>
                            </TextField>
                        </FormControl>
                    </Tooltip>
                    <Tooltip
                        title='The nickname of the provider to which the device is assigned'
                        arrow
                        placement='top'
                    >
                        <FormControl fullWidth variant='outlined' required>
                            <TextField
                                style={{ width: '100%' }}
                                variant='outlined'
                                value={provider}
                                onChange={(e) => setProvider(e.target.value)}
                                select
                                label='Provider'
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
                    </Tooltip>
                    <Button
                        variant='contained'
                        type='submit'
                        style={{
                            backgroundColor: '#2f3b26',
                            color: '#f4e6cd',
                            fontWeight: 'bold',
                            boxShadow: 'none',
                            height: '40px'
                        }}
                        disabled={loading || updateDeviceStatus === 'success' || updateDeviceStatus === 'error'}
                    >
                        {loading ? (
                            <CircularProgress size={25} style={{ color: '#f4e6cd' }} />
                        ) : updateDeviceStatus === 'success' ? (
                            <CheckIcon size={25} style={{ color: '#f4e6cd', stroke: '#f4e6cd', strokeWidth: 2 }} />
                        ) : updateDeviceStatus === 'error' ? (
                            <CloseIcon size={25} style={{ color: 'red', stroke: 'red', strokeWidth: 2 }} />
                        ) : (
                            'Update device'
                        )}
                    </Button>
                    <Button
                        variant='contained'
                        style={{
                            backgroundColor: '#2f3b26',
                            color: '#f4e6cd',
                            fontWeight: 'bold',
                            boxShadow: 'none',
                            height: '40px'
                        }}
                        onClick={handleReprovisionDevice}
                        disabled={reprovisionLoading || reprovisionDeviceStatus === 'success' || reprovisionDeviceStatus === 'error'}
                    >
                        {reprovisionLoading ? (
                            <CircularProgress size={25} style={{ color: '#f4e6cd' }} />
                        ) : reprovisionDeviceStatus === 'success' ? (
                            <CheckIcon size={25} style={{ color: '#f4e6cd', stroke: '#f4e6cd', strokeWidth: 2 }} />
                        ) : reprovisionDeviceStatus === 'error' ? (
                            <CloseIcon size={25} style={{ color: 'red', stroke: 'red', strokeWidth: 2 }} />
                        ) : (
                            'Re-provision device'
                        )}
                    </Button>
                    <Button
                        onClick={(event) => showDeleteDeviceAlert(event)}
                        style={{
                            backgroundColor: 'orange',
                            color: '#2f3b26',
                            fontWeight: 'bold',
                            boxShadow: 'none',
                            height: '40px'
                        }}
                    >Delete device</Button>
                </Stack>
            </form>
        </Box>
    )
}