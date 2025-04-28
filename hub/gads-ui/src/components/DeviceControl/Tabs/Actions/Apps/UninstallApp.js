import { Box, Button, CircularProgress, FormControl, MenuItem, Select, Stack } from '@mui/material'
import InstallMobileIcon from '@mui/icons-material/InstallMobile'
import './InstallApp.css'
import { useEffect, useState } from 'react'
import { api } from '../../../../../services/api.js'
import { useSnackbar } from '../../../../../contexts/SnackBarContext.js'

export default function UninstallApp({ udid, installedApps }) {
    const { showSnackbar } = useSnackbar()
    const [selectedAppUninstall, setSelectedAppUninstall] = useState('no-app')
    const [uninstallButtonDisabled, setUninstallButtonDisabled] = useState(true)
    const [isUninstalling, setIsUninstalling] = useState(false)
    const [installedAppsList, setInstalledAppsList] = useState(installedApps)

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
        const url = `/device/${udid}/uninstallApp`

        const body = {
            app: selectedAppUninstall
        }

        api.post(url, body)
            .then((response) => {
                setInstalledAppsList(response.data.result)
                setSelectedAppUninstall('no-app')
                setIsUninstalling(false)
            })
            .catch(error => {
                if (error.response) {
                    showCustomSnackbarError(`Failed to uninstall '${selectedAppUninstall}'`)
                    setIsUninstalling(false)
                }
                setIsUninstalling(false)
            })
    }

    const showCustomSnackbarError = (message) => {
        showSnackbar({
            message: message,
            severity: 'error',
            duration: 3000,
        })
    }

    return (
        <Box style={{ width: '300px' }}>
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
                            value={selectedAppUninstall}
                            id='app-select'
                            onChange={(event) => handleUninstallChange(event)}
                        >
                            <MenuItem
                                className='select-items'
                                value={selectedAppUninstall}
                            >No app selected</MenuItem>
                            {
                                installedAppsList.map((installedApp) => {
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
                            backgroundColor: '#2f3b26',
                            color: '#9ba984',
                            fontWeight: 'bold',
                            width: '260px'
                        }}
                    >
                        {isUninstalling ? (
                            <CircularProgress id='progress-indicator' size={30} />
                        ) : (
                            'Uninstall'
                        )}
                    </Button>

                </Box>
            </Stack>
        </Box>
    )
}