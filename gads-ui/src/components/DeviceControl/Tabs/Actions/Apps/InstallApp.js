import { Box, Button, CircularProgress, FormControl, MenuItem, Select, Stack } from "@mui/material";
import InstallMobileIcon from '@mui/icons-material/InstallMobile';
import './InstallApp.css'
import { useState } from "react";

export default function InstallApp({ installableApps }) {
    const [selectedApp, setSelectedApp] = useState('no-app')
    const [buttonDisabled, setButtonDisabled] = useState(true)

    function handleSelectChange(event) {
        const app = event.target.value
        if (app.includes('no-app')) {
            setButtonDisabled(true)
        } else {
            setButtonDisabled(false)
        }
        setSelectedApp(app)
    }

    return (
        <Box style={{ width: '300px' }}>
            <Stack
                alignItems='center'
            >
                <h3>Install app</h3>
                <Box style={{ width: '260px' }}>
                    <FormControl id='form-control'>
                        <Select
                            defaultValue='no-app'
                            id='app-select'
                            onChange={(event) => handleSelectChange(event)}
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
                    <Button startIcon={<InstallMobileIcon />} id='install-button' variant='contained' disabled={buttonDisabled}>Install</Button>
                    {/* {isUploading &&
                <CircularProgress id='progress-indicator' size={30} />
            } */}
                </Box>
            </Stack>
        </Box >
    )
}