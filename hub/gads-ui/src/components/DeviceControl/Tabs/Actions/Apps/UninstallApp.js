import { Box, Button, CircularProgress, FormControl, MenuItem, Select, Stack } from "@mui/material";
import InstallMobileIcon from '@mui/icons-material/InstallMobile';
import './InstallApp.css'
import { useContext, useState } from "react";
import { Auth } from "../../../../../contexts/Auth";
import { api } from '../../../../../services/api.js'

export default function UninstallApp({ udid, installedApps }) {
    const [selectedAppUninstall, setSelectedAppUninstall] = useState('no-app')
    const [uninstallButtonDisabled, setUninstallButtonDisabled] = useState(true)
    const [isUninstalling, setIsUninstalling] = useState(false)
    const {logout} = useContext(Auth)

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

        api.post(url, body)
            .then(() => {
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
            });
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
                            backgroundColor: "#2f3b26",
                            color: "#9ba984",
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