import { Box, Button, CircularProgress, FormControl, MenuItem, Select, Stack } from "@mui/material";
import InstallMobileIcon from '@mui/icons-material/InstallMobile';
import './InstallApp.css'
import { useState } from "react";

export default function InstallApp({ installableApps, installedApps }) {
    const [selectedApp, setSelectedApp] = useState('no-app')
    const [installButtonDisabled, setInstallButtonDisabled] = useState(true)
    const [uninstallButtonDisabled, setUninstallButtonDisabled] = useState(true)
    const [isInstalling, setIsInstalling] = useState(false)
    const [isUninstalling, setIsUninstalling] = useState(false)

    function handleInstallChange(event) {
        const app = event.target.value
        if (app.includes('no-app')) {
            setInstallButtonDisabled(true)
        } else {
            setInstallButtonDisabled(false)
        }
        setSelectedApp(app)
    }

    function handleUninstallChange(event) {
        const app = event.target.value
        if (app.includes('no-app')) {
            setUninstallButtonDisabled(true)
        } else {
            setUninstallButtonDisabled(false)
        }
        setSelectedApp(app)
    }

    return (
        <Box style={{ width: '300px' }}>
            <Stack
                alignItems='center'
                height='50%'
            >
                <h3>Install app</h3>
                <Box style={{ width: '260px' }}>
                    <FormControl id='form-control'>
                        <Select
                            defaultValue='no-app'
                            id='app-select'
                            onChange={(event) => handleInstallChange(event)}
                        >
                            <MenuItem className='select-items' value='no-app'>No app selected</MenuItem>
                            {
                                installableApps.map((installableApp) => {
                                    return (
                                        <MenuItem className='select-items' value={installableApp}> {installableApp}</MenuItem>
                                    )
                                })
                            }
                        </Select>
                    </FormControl>
                </Box>
                <Box id='install-box'>
                    <Button startIcon={<InstallMobileIcon />} id='install-button' variant='contained' disabled={installButtonDisabled}>Install</Button>
                    {isInstalling &&
                        <CircularProgress id='progress-indicator' size={30} />
                    }
                </Box>
            </Stack>
            <Stack
                alignItems='center'
            >
                <h3>Uninstall app</h3>
                <Box style={{ width: '260px' }}>
                    <FormControl id='form-control'>
                        <Select
                            defaultValue='no-app'
                            id='app-select'
                            onChange={(event) => handleUninstallChange(event)}
                        >
                            <MenuItem className='select-items' value='no-app'>No app selected</MenuItem>
                            {
                                installedApps.map((installedApp) => {
                                    return (
                                        <MenuItem className='select-items' value={installedApp}> {installedApp}</MenuItem>
                                    )
                                })
                            }
                        </Select>
                    </FormControl>
                </Box>
                <Box id='install-box'>
                    <Button startIcon={<InstallMobileIcon />} id='install-button' variant='contained' disabled={uninstallButtonDisabled}>Uninstall</Button>
                    {isUninstalling &&
                        <CircularProgress id='progress-indicator' size={30} />
                    }
                </Box>
            </Stack>
        </Box >
    )
}