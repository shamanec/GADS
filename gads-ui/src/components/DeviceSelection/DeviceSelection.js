import React, { useEffect, useState, useContext } from 'react'
import './DeviceSelection.css'
import Alert from '@mui/material/Alert';
import Snackbar from '@mui/material/Snackbar';
import Box from '@mui/material/Box';
import TabPanel from '@mui/lab/TabPanel';
import TabContext from '@mui/lab/TabContext';
import Grid from '@mui/material/Grid';
import Stack from '@mui/material/Stack';
import Divider from '@mui/material/Divider';
import { OSFilterTabs, DeviceSearch } from './Filters'
import { DeviceBox } from './Device'
import { Auth } from '../../contexts/Auth';

export default function DeviceSelection() {
    // States
    const [devices, setDevices] = useState([]);
    const [showAlert, setShowAlert] = useState(false);
    const [timeoutId, setTimeoutId] = useState(null);

    let devicesSocket = null;
    let vertical = 'bottom'
    let horizontal = 'center'

    const open = true

    // Authentication and session control
    const [authToken, , logout] = useContext(Auth)

    function CheckServerHealth() {
        let url = `/health`

        fetch(url, {
            method: 'GET',
            headers: {
                'X-Auth-Token': authToken
            }
        })
            .then((response) => {
                if (!response.ok) {
                    logout()
                }
            })
            .catch((e) => {
                logout()
                console.log(e)
            })
    }

    // Show a snackbar alert if device is unavailable
    function presentDeviceUnavailableAlert() {
        // Present the alert
        setShowAlert(true);
        // Clear the previous timeout if it exists
        clearTimeout(timeoutId);
        // Set a new timeout for the alert
        setTimeoutId(
            setTimeout(() => {
                setShowAlert(false);
            }, 3000)
        );
    }

    useEffect(() => {
        CheckServerHealth()

        if (devicesSocket) {
            devicesSocket.close()
        }
        let url = `ws://${window.location.host}/available-devices`
        devicesSocket = new WebSocket(url);

        devicesSocket.onmessage = (message) => {
            let devicesJson = JSON.parse(message.data)
            localStorage.setItem("devices-data", devicesJson)

            setDevices(devicesJson);
            devicesJson.forEach((device) => {
                localStorage.setItem(device.udid, JSON.stringify(device))
            })
        }

        // If component unmounts close the websocket connection
        return () => {
            if (devicesSocket) {
                console.log('component unmounted')
                devicesSocket.close()
            }
        }
    }, [])

    return (
        <div id='top-wrapper'>
            <div id='selection-wrapper'>
                <OSSelection devices={devices} handleAlert={presentDeviceUnavailableAlert} />
                {showAlert && (
                    <Snackbar
                        anchorOrigin={{ vertical, horizontal }}
                        open={open}
                        key='bottomcenter'
                    >
                        <Alert severity="error">
                            Device is unavailable
                        </Alert>
                    </Snackbar>
                )}
            </div>
        </div>
    )
}

function OSSelection({ devices, handleAlert }) {
    const [currentTabIndex, setCurrentTabIndex] = useState(0);

    const handleTabChange = (e, tabIndex) => {
        setCurrentTabIndex(tabIndex);
        const searchInput = document.getElementById('search-input')
        searchInput.value = ''
    };

    return (
        <TabContext value='{currentTabIndex}'>
            <Box sx={{ display: 'flex', flexDirection: 'row' }}>
                <Stack
                    direction='column'
                    divider={<Divider orientation='vertical' flexItem />}
                    spacing={2}
                    alignItems='center'
                    className='filters-stack'
                    sx={{ height: '500px', backgroundColor: '#E0D8C0', borderRadius: '10px' }}
                >
                    <OSFilterTabs currentTabIndex={currentTabIndex} handleTabChange={handleTabChange}></OSFilterTabs>
                    <DeviceSearch keyUpFilterFunc={deviceSearch}></DeviceSearch>
                </Stack>
                <TabPanel value='{currentTabIndex}'>
                    <Grid id='devices-container' container spacing={2}>
                        {
                            devices.map((device) => {
                                if (currentTabIndex === 0) {
                                    return (
                                        <DeviceBox device={device} handleAlert={handleAlert} />
                                    )

                                } else if (currentTabIndex === 1 && device.os === 'android') {
                                    return (
                                        <DeviceBox device={device} handleAlert={handleAlert} />
                                    )

                                } else if (currentTabIndex === 2 && device.os === 'ios') {
                                    return (
                                        <DeviceBox device={device} handleAlert={handleAlert} />
                                    )
                                }
                            })
                        }
                    </Grid>
                </TabPanel>
            </Box>
        </TabContext>
    )
}

function deviceSearch() {
    var input = document.getElementById('search-input');
    var filter = input.value.toUpperCase();
    let grid = document.getElementById('devices-container')
    let deviceBoxes = grid.getElementsByClassName('device-box')
    for (let i = 0; i < deviceBoxes.length; i++) {
        let shouldDisplay = false
        var filterables = deviceBoxes[i].getElementsByClassName('filterable')
        for (let j = 0; j < filterables.length; j++) {
            var filterable = filterables[j]
            var txtValue = filterable.textContent || filterable.innerText;
            if (txtValue.toUpperCase().indexOf(filter) > -1) {
                shouldDisplay = true
            }
        }

        if (shouldDisplay) {
            deviceBoxes[i].style.display = '';
        } else {
            deviceBoxes[i].style.display = 'none';
        }
    }
}
