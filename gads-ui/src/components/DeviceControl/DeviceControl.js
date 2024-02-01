import { useParams } from 'react-router-dom';
import { useNavigate } from 'react-router-dom';
import StreamCanvas from './StreamCanvas'
import { Skeleton, Stack } from '@mui/material';
import { Button } from '@mui/material';
import TabularControl from './Tabs/TabularControl';
import { useContext, useEffect, useState } from 'react';
import { Auth } from '../../contexts/Auth';
import axios from 'axios'
import { DialogProvider } from './SessionDialogContext';

export default function DeviceControl() {
    const { authToken, signOut } = useContext(Auth)
    const { id } = useParams();
    const navigate = useNavigate();
    const [deviceData, setDeviceData] = useState(null)
    const [isLoading, setIsLoading] = useState(true)

    let url = `/device/${id}/info`
    let in_use_socket = null
    useEffect(() => {
        axios.get(url, {
            headers: {
                'X-Auth-Token': authToken
            }
        })
            .then((response) => {
                setDeviceData(response.data)
            })
            .catch(error => {
                if (error.response) {
                    if (error.response.status === 401) {
                        signOut()
                        return
                    }
                }
                console.log('Failed getting providers data' + error)
                navigate('/devices');
                return
            });

        if (in_use_socket) {
            in_use_socket.close()
        }
        let socketUrl = `ws://${window.location.host}/devices/control/${id}/in-use`
        in_use_socket = new WebSocket(socketUrl);
        if (in_use_socket.readyState === WebSocket.OPEN) {
            in_use_socket.send('ping');
        }
        const pingInterval = setInterval(() => {
            if (in_use_socket.readyState === WebSocket.OPEN) {
                in_use_socket.send('ping');
            }
        }, 1000);

        setInterval(() => {
            setIsLoading(false)
        }, 2000);

        return () => {
            if (in_use_socket) {
                console.log('component unmounted, clearing itnerval and closing socket')
                clearInterval(pingInterval)
                in_use_socket.close()
            }
        }

    }, [])

    const handleBackClick = () => {
        navigate('/devices');
    };

    return (
        <DialogProvider>
            <div>
                <div className='back-button-bar' style={{
                    marginBottom: '10px',
                    marginTop: '10px'
                }}>
                    <Button
                        variant="contained"
                        onClick={handleBackClick}
                        style={{ marginLeft: "20px" }}
                    >Back to devices</Button>
                </div>
                {
                    isLoading ? (
                        <Stack
                            direction='row'
                            spacing={2}
                            style={{
                                marginLeft: "20px"
                            }}
                        >
                            <Skeleton
                                variant="rounded"
                                style={{
                                    backgroundColor: 'gray',
                                    animationDuration: '1s',
                                    height: '950px',
                                    width: '500px',
                                    borderRadius: '30px'
                                }}
                            />
                            <Skeleton
                                variant="rounded"
                                style={{
                                    backgroundColor: 'gray',
                                    animationDuration: '1s',
                                    height: '850px',
                                    width: '100%',
                                    marginRight: '10px'
                                }}
                            />
                        </Stack>
                    ) : (
                        <>
                            <Stack
                                direction='row'
                                spacing={2}
                                style={{
                                    marginLeft: "20px"
                                }}
                            >
                                <StreamCanvas
                                    deviceData={deviceData}
                                ></StreamCanvas>
                                <TabularControl
                                    deviceData={deviceData}
                                ></TabularControl>
                            </Stack>
                        </>
                    )
                }
            </div>
        </DialogProvider>
    )
}