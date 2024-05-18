import { Box, Button, CircularProgress, FormControl, MenuItem, Select, Stack } from "@mui/material";
import InstallMobileIcon from '@mui/icons-material/InstallMobile';
import './InstallApp.css'
import { useContext, useState } from "react";
import axios from 'axios'
import { Auth } from "../../../../../contexts/Auth";

export default function InstallApp({ udid, installableApps, installedApps }) {
    const [selectedAppUninstall, setSelectedAppUninstall] = useState('no-app')
    const [selectedAppInstall, setSelectedAppInstall] = useState('no-app')
    const [installButtonDisabled, setInstallButtonDisabled] = useState(true)
    const [uninstallButtonDisabled, setUninstallButtonDisabled] = useState(true)
    const [isInstalling, setIsInstalling] = useState(false)
    const [isUninstalling, setIsUninstalling] = useState(false)
    const [authToken, , , , logout] = useContext(Auth)

    function handleInstallChange(event) {
        const app = event.target.value
        if (app.includes('no-app')) {
            setInstallButtonDisabled(true)
        } else {
            setInstallButtonDisabled(false)
        }
        setSelectedAppInstall(app)
    }

    function handleUninstallChange(event) {
        const app = event.target.value
        if (app.includes('no-app')) {
            setUninstallButtonDisabled(true)
        } else {
            setUninstallButtonDisabled(false)
        }
        setSelectedAppUninstall(app)
    }

    function handleUninstall() {
        setIsUninstalling(true)
        const url = `/device/${udid}/uninstallApp`;

        const body = `{
            "app": "` + selectedAppUninstall + `"
        } `

        axios.post(url, body, {
            headers: {
                'X-Auth-Token': authToken
            }
        })
            .then((response) => {
                setIsUninstalling(false)
            })
            .catch(error => {
                if (error.response) {
                    if (error.response.status === 401) {
                        logout()
                        return
                    }
                    setIsUninstalling(false)
                }
                setIsUninstalling(false)
                console.log('Failed uploading file - ' + error)
            });
    }

    function handleInstall() {
        setIsInstalling(true)
        const url = `/device/${udid}/installApp`;

        const body = `{
            "app": "` + selectedAppInstall + `"
        } `

        axios.post(url, body, {
            headers: {
                'X-Auth-Token': authToken
            }
        })
            .then((response) => {
                setIsInstalling(false)
            })
            .catch(error => {
                if (error.response) {
                    if (error.response.status === 401) {
                        logout()
                        return
                    }
                    setIsInstalling(false)
                }
                setIsInstalling(false)
                console.log('Failed uploading file - ' + error)
            });
    }

    return (
        <Box style={{ width: '300px' }}>
            <Stack
                alignItems='center'
                height='50%'
            >
                <h3>Install app</h3>
                <Box style={{ width: '260px' }}>
                    <FormControl
                        id='form-control'
                    >
                        <Select
                            defaultValue='no-app'
                            id='app-select'
                            onChange={(event) => handleInstallChange(event)}
                        >
                            <MenuItem
                                className='select-items'
                                value='no-app'
                            >No app selected</MenuItem>
                            {
                                installableApps.map((installableApp) => {
                                    return (
                                        <MenuItem
                                            className='select-items'
                                            value={installableApp}
                                        > {installableApp}</MenuItem>
                                    )
                                })
                            }
                        </Select>
                    </FormControl>
                </Box>
                <Box
                    id='install-box'
                >
                    <Button
                        onClick={handleInstall}
                        startIcon={<InstallMobileIcon />}
                        id='install-button'
                        variant='contained'
                        disabled={installButtonDisabled}
                        style={{
                            backgroundColor: "#0c111e",
                            color: "#78866B",
                            fontWeight: "bold"
                        }}
                    >Install</Button>
                    {isInstalling &&
                        <CircularProgress id='progress-indicator' size={30} />
                    }
                </Box>
            </Stack>
            <Stack
                alignItems='center'
            >
                <h3>Uninstall app</h3>
                <Box
                    style={{
                        width: '260px'
                    }}
                >
                    <FormControl
                        id='form-control'
                    >
                        <Select
                            defaultValue='no-app'
                            id='app-select'
                            onChange={(event) => handleUninstallChange(event)}
                        >
                            <MenuItem
                                className='select-items'
                                value='no-app'
                            >No app selected</MenuItem>
                            {
                                installedApps.map((installedApp) => {
                                    return (
                                        <MenuItem
                                            className='select-items'
                                            value={installedApp}
                                        > {installedApp}</MenuItem>
                                    )
                                })
                            }
                        </Select>
                    </FormControl>
                </Box>
                <Box id='install-box'>
                    <Button
                        onClick={handleUninstall}
                        startIcon={<InstallMobileIcon />}
                        id='install-button'
                        variant='contained'
                        disabled={uninstallButtonDisabled}
                        style={{
                            backgroundColor: "#0c111e",
                            color: "#78866B",
                            fontWeight: "bold"
                        }}
                    >Uninstall</Button>
                    {isUninstalling &&
                        <CircularProgress id='progress-indicator' size={30} />
                    }
                </Box>
            </Stack>
        </Box >
    )
}