import React, { useEffect, useState } from 'react'
import './DeviceTable.css'
import Alert from '@mui/material/Alert';
import Snackbar from '@mui/material/Snackbar';
import { useNavigate } from 'react-router-dom';
import Tabs from '@mui/material/Tabs';
import Tab from '@mui/material/Tab';
import Box from '@mui/material/Box';
import TabPanel from '@mui/lab/TabPanel';
import TabContext from '@mui/lab/TabContext';
import Grid from '@mui/material/Grid';

export default function DeviceTable() {
    let devicesSocket = null;
    const [devices, setDevices] = useState([]);
    const [showAlert, setShowAlert] = useState(false);
    let vertical = 'bottom'
    let horizontal = 'center'
    const [timeoutId, setTimeoutId] = useState(null);
    const open = true

    localStorage.clear()

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
        if (devicesSocket) {
            devicesSocket.close()
        }
        devicesSocket = new WebSocket('ws://192.168.1.28:10000/available-devices');

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
        <div>
            <OSSelection devices={devices} />
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
    )
}

function OSSelection({ devices }) {
    const [currentTabIndex, setCurrentTabIndex] = useState(0);

    const handleTabChange = (e, tabIndex) => {
        setCurrentTabIndex(tabIndex);
    };

    return (
        <TabContext value='{currentTabIndex}'>
            <Box>
                <Tabs value={currentTabIndex} onChange={handleTabChange}>
                    <Tab label="All" />
                    <Tab label="Android" />
                    <Tab label="iOS" />
                </Tabs>
                <TabPanel value='{currentTabIndex}'>
                    <Box sx={{ flexGrow: 1 }}></Box>
                    <Grid container spacing={2}>
                        {
                            devices.map((device, index) => {
                                if (currentTabIndex === 0) {
                                    return (
                                        <DeviceBox device={device} />
                                    )

                                } else if (currentTabIndex === 1 && device.os === "android") {
                                    return (
                                        <DeviceBox device={device} />
                                    )

                                } else if (currentTabIndex === 2 && device.os === "ios") {
                                    return (
                                        <DeviceBox device={device} />
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

function filterDevices() {
    var input = document.getElementById("search-input");
    var filter = input.value.toUpperCase();
    let grid = document.getElementById('devices-container')
    let deviceBoxes = grid.getElementsByClassName('device-box')
    for (let i = 0; i < deviceBoxes.length; i++) {
        let shouldDisplay = false
        var filterables = deviceBoxes[i].getElementsByClassName("filterable")
        for (let j = 0; j < filterables.length; j++) {
            var filterable = filterables[j]
            var txtValue = filterable.textContent || filterable.innerText;
            if (txtValue.toUpperCase().indexOf(filter) > -1) {
                shouldDisplay = true
            }
        }

        if (shouldDisplay) {
            deviceBoxes[i].style.display = "";
        } else {
            deviceBoxes[i].style.display = "none";
        }
    }
}

function DeviceBox({ device, handleAlert }) {
    let img_src = device.os === 'android' ? './images/default-android.png' : './images/default-apple.png'

    return (
        <div className='device-box' data-id={device.udid}>
            <div>
                <img className="deviceImage" src={img_src}>
                </img>
            </div>
            <div className='filterable info'>{device.model}</div>
            <div className='filterable info'>{device.os_version}</div>
            <div className='device-buttons-container'>
                <UseButton device={device} handleAlert={handleAlert} />
                <button className='device-buttons'>Details</button>
            </div>
        </div>
    )
}

function UseButton({ device, handleAlert }) {
    // Difference between current time and last time the device was reported as healthy
    // let healthyDiff = (Date.now() - device.last_healthy_timestamp)
    const [loading, setLoading] = useState(false);
    const navigate = useNavigate();

    function handleUseButtonClick() {
        setLoading(true);
        const url = `http://${device.host_address}:10000/device/${device.udid}/health`;
        fetch(url)
            .then((response) => {
                if (!response.ok) {
                    throw new Error('Network response was not ok');
                } else {
                    navigate('/devices/control/' + device.udid);
                }
            })
            .catch((error) => {
                handleAlert()
                console.error('Error fetching data:', error);
            })
            .finally(() => {
                setTimeout(() => {
                    setLoading(false);
                }, 2000);
            });
    }

    const buttonDisabled = loading || !device.connected;

    if (device.connected === true) {
        return (
            <button className='device-buttons' onClick={handleUseButtonClick} disabled={buttonDisabled}>
                {loading ? <span className="spinner"></span> : 'Use'}
            </button>

        );
    } else {
        return (
            <button className='device-buttons' disabled>N/A</button>
        );
    }
}
