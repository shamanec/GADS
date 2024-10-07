import React, { useEffect, useState, useContext } from 'react'
import './DeviceSelection.css'
import Box from '@mui/material/Box';
import TabPanel from '@mui/lab/TabPanel';
import TabContext from '@mui/lab/TabContext';
import Grid from '@mui/material/Grid';
import Stack from '@mui/material/Stack';
import Divider from '@mui/material/Divider';
import { OSFilterTabs, DeviceSearch } from './Filters'
import { Auth } from '../../contexts/Auth';
import { api } from '../../services/api.js'
import DeviceBox from "./DeviceBox";

export default function DeviceSelection() {
    const open = true
    const [devices, setDevices] = useState([]);
    const { logout } = useContext(Auth)

    function CheckServerHealth() {
        let url = `/health`

        api.get(url)
            .then(response => {
                if (response.status !== 200) {
                    console.log("Got bad response on checking server health, logging out")
                    logout()
                }
            })
            .catch(e => {
                console.log("Got error on checking server health")
                console.log(e)
                logout()
            })
    }

    useEffect(() => {
        CheckServerHealth()

        // Use specific full address for local development, proxy does not seem to work okay
        // const evtSource = new EventSource(`http://192.168.1.41:10000/available-devices`);
        const evtSource = new EventSource(`/available-devices`);

        evtSource.onmessage = (message) => {
            let devicesJson = JSON.parse(message.data)
            setDevices(devicesJson);
        }

        // If component unmounts close the websocket connection
        return () => {
            if (evtSource) {
                evtSource.close()
            }
        }
    }, [])

    return (
        <div
            id='top-wrapper'
        >
            <div
                id='selection-wrapper'
            >
                <OSSelection
                    devices={devices}
                />
            </div>
        </div>
    )
}

function OSSelection({ devices }) {
    const [currentTabIndex, setCurrentTabIndex] = useState(0);

    const handleTabChange = (e, tabIndex) => {
        setCurrentTabIndex(tabIndex);
        const searchInput = document.getElementById('search-input')
        searchInput.value = ''
    };

    return (
        <TabContext value='{currentTabIndex}'>
            <Box
                sx={{
                    display: 'flex',
                    flexDirection: 'row'
                }}
            >
                <Stack
                    direction='column'
                    divider={<Divider orientation='vertical' flexItem />}
                    spacing={2}
                    alignItems='center'
                    className='filters-stack'
                    sx={{
                        height: '200px',
                        backgroundColor: '#9ba984',
                        borderRadius: '10px'
                    }}
                >
                    <OSFilterTabs
                        currentTabIndex={currentTabIndex}
                        handleTabChange={handleTabChange}
                    ></OSFilterTabs>
                    <DeviceSearch
                        keyUpFilterFunc={deviceSearch}
                    ></DeviceSearch>
                </Stack>
                {devices.length === 0 ? (
                    <Box
                        style={{
                            backgroundColor: '#9ba984',
                            width: '100%',
                            height: '800px',
                            borderRadius: '10px',
                            margin: '10px',
                            display: 'flex',
                            justifyContent: 'center',
                            alignItems: 'center'
                        }}
                    >
                        <div
                            style={{
                                fontSize: '30px',
                                fontFamily: 'Verdana'
                            }}
                        >No device data available</div>
                    </Box>
                ) : (
                    <TabPanel
                        value='{currentTabIndex}'
                        style={{ height: "80vh", overflowY: "auto" }}
                    >
                        <Grid
                            id='devices-container'
                            container
                            spacing={2}
                            style={{
                                marginBottom: '10px'
                            }}
                        >
                            {
                                devices.map((device) => {
                                    if (currentTabIndex === 0) {
                                        return (
                                            <Grid item>
                                                <DeviceBox
                                                    device={device}
                                                />
                                            </Grid>
                                        )

                                    } else if (currentTabIndex === 1 && device.info.os === 'android') {
                                        return (
                                            <Grid item>
                                                <DeviceBox
                                                    device={device}
                                                />
                                            </Grid>
                                        )

                                    } else if (currentTabIndex === 2 && device.info.os === 'ios') {
                                        return (
                                            <Grid item>
                                                <DeviceBox
                                                    device={device}
                                                />
                                            </Grid>
                                        )
                                    }
                                })
                            }
                        </Grid>
                    </TabPanel>
                )}
            </Box>
        </TabContext >
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
