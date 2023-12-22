import { useEffect } from "react"
import ShowFailedSessionAlert from "./SessionAlert"
import { Auth } from "../../contexts/Auth"
import { useContext } from "react"
import './StreamCanvas.css'
import { Button, Divider, Grid, Stack } from "@mui/material"
import HomeIcon from '@mui/icons-material/Home';
import LockOpenIcon from '@mui/icons-material/LockOpen';
import LockIcon from '@mui/icons-material/Lock';

export default function StreamCanvas({ deviceData }) {
    const [authToken, login, logout] = useContext(Auth)

    let deviceX = parseInt(deviceData.Device.screen_width, 10)
    let deviceY = parseInt(deviceData.Device.screen_height, 10)
    let screen_ratio = deviceX / deviceY
    let canvasHeight = 850
    let canvasWidth = 850 * screen_ratio

    const streamData = {
        udid: deviceData.Device.udid,
        deviceX: deviceX,
        deviceY: deviceY,
        screen_ratio: screen_ratio,
        canvasHeight: canvasHeight,
        canvasWidth: canvasWidth
    }

    let streamSocket = null;
    useEffect(() => {
        if (streamSocket) {
            streamSocket.close()
        }

        if (deviceData.Device.os === 'ios') {
            streamSocket = new WebSocket(`ws://${window.location.host}/device/${deviceData.Device.udid}/ios-stream`);
        } else {
            streamSocket = new WebSocket(`ws://${window.location.host}/device/${deviceData.Device.udid}/android-stream`);
        }

        let imgElement = document.getElementById('image-stream')
        streamSocket.onmessage = (message) => {
            streamSocket.onmessage = function (event) {
                const imageURL = URL.createObjectURL(event.data);
                imgElement.src = imageURL

                imgElement.onload = () => {
                    URL.revokeObjectURL(imageURL);
                };
            }
        }

        // If component unmounts close the websocket connection
        return () => {
            if (streamSocket) {
                console.log('stream unmounted')
                streamSocket.close()
            }
        }
    }, [])

    return (
        <div
            id='phone-imitation'
        >
            <div
                id="stream-div"
            >
                <Canvas
                    canvasWidth={canvasWidth}
                    canvasHeight={canvasHeight}
                    authToken={authToken}
                    logout={logout}
                    streamData={streamData}
                ></Canvas>
                <Stream
                    canvasWidth={canvasWidth}
                    canvasHeight={canvasHeight}
                ></Stream>
            </div>
            <Divider></Divider>
            <Grid height='50px' display='flex' justifyContent='center' style={{ marginTop: '10px' }}>
                <Button onClick={() => homeButton(authToken, deviceData)} className='canvas-buttons' startIcon={<HomeIcon />} variant='contained' style={{ borderBottomLeftRadius: '25px' }}>Home</Button>
                <Button onClick={() => lockButton(authToken, deviceData)} className='canvas-buttons' startIcon={<LockIcon />} variant='contained' >Lock</Button>
                <Button onClick={() => unlockButton(authToken, deviceData)} className='canvas-buttons' startIcon={<LockOpenIcon />} variant='contained' style={{ borderBottomRightRadius: '25px' }}>Unlock</Button>
            </Grid>
        </div >

    )
}

function Canvas({ authToken, logout, streamData }) {
    var tapStartAt = 0
    var coord1;
    var coord2;

    function getCursorCoordinates(event) {
        const rect = event.currentTarget.getBoundingClientRect()
        const x = event.clientX - rect.left
        const y = event.clientY - rect.top
        return [x, y];
    }

    function handleMouseDown(event) {
        tapStartAt = (new Date()).getTime()
        coord1 = getCursorCoordinates(event)
        console.log('Tapped on ' + coord1)
    }

    function handleMouseUp(event) {
        coord2 = getCursorCoordinates(event)
        console.log('Released on' + coord2)
        // get the time of finishing the click on the canvas
        var tapEndAt = (new Date()).getTime()

        var mouseEventsTimeDiff = tapEndAt - tapStartAt

        // if the difference of time between click down and click up is more than 500ms assume it is a swipe, not a tap
        // to allow flick swipes we also check the difference between the gesture coordinates
        // x1, y1 = mousedown coordinates
        // x2, y2 = mouseup coordinates
        // if x2 > x1*1.1 - it is probably a swipe left to right
        // if x2 < x1*0.9 - it is probably a swipe right to left
        // if y2 < y1*0.9 - it is probably a swipe bottom to top
        // if y2 > y1*1.1 - it is probably a swipe top to bottom
        if (mouseEventsTimeDiff > 500 || coord2[0] > coord1[0] * 1.1 || coord2[0] < coord1[0] * 0.9 || coord2[1] < coord1[1] * 0.9 || coord2[1] > coord1[1] * 1.1) {
            swipeCoordinates(authToken, logout, coord1, coord2, streamData)
        } else {
            tapCoordinates(authToken, logout, coord1, streamData)
        }
    }

    return (
        <canvas
            id="actions-canvas"
            width={streamData.canvasWidth + 'px'}
            height={streamData.canvasHeight + 'px'}
            onMouseDown={handleMouseDown}
            onMouseUp={handleMouseUp}
            style={{ position: "absolute" }}
        ></canvas>
    )
}

function Stream({ canvasWidth, canvasHeight }) {
    return (
        <img
            id="image-stream"
            width={canvasWidth + 'px'}
            height={canvasHeight + 'px'}
            style={{ display: 'block' }}
        ></img>
    )
}

// tap with WDA using coordinates
function tapCoordinates(authToken, logout, pos, streamData) {
    // set initial x and y tap coordinates
    let x = pos[0]
    let y = pos[1]

    // if the stream height 
    if (streamData.canvasHeight != streamData.deviceY) {
        x = (x / streamData.canvasWidth) * streamData.deviceX
        y = (y / streamData.canvasHeight) * streamData.deviceY
    }

    let jsonData = JSON.stringify({
        "x": x,
        "y": y
    })

    let deviceURL = `/device/${streamData.udid}`

    fetch(deviceURL + "/tap", {
        method: 'POST',
        body: jsonData,
        headers: {
            'Content-type': 'application/json',
            'X-Auth-Token': authToken
        }
    })
        .then(response => {
            if (response.status === 404) {
                ShowFailedSessionAlert(deviceURL)
                return
            }

            if (response.status === 401) {
                logout()
            }
        })
        .catch(function (error) {
            ShowFailedSessionAlert(deviceURL)
        })
}

function swipeCoordinates(authToken, logout, coord1, coord2, streamData) {
    var firstCoordX = coord1[0]
    var firstCoordY = coord1[1]
    var secondCoordX = coord2[0]
    var secondCoordY = coord2[1]

    // if the stream height 
    if (streamData.canvasHeight != streamData.deviceY) {
        firstCoordX = (firstCoordX / streamData.canvasWidth) * streamData.deviceX
        firstCoordY = (firstCoordY / streamData.canvasHeight) * streamData.deviceY
        secondCoordX = (secondCoordX / streamData.canvasWidth) * streamData.deviceX
        secondCoordY = (secondCoordY / streamData.canvasHeight) * streamData.deviceY
    }

    let jsonData = JSON.stringify({
        "x": firstCoordX,
        "y": firstCoordY,
        "endX": secondCoordX,
        "endY": secondCoordY
    })

    let deviceURL = `/device/${streamData.udid}`

    fetch(deviceURL + "/swipe", {
        method: 'POST',
        body: jsonData,
        headers: {
            'Content-type': 'application/json',
            'X-Auth-Token': authToken
        }
    })
        .then(response => {
            if (response.status === 404) {
                ShowFailedSessionAlert(deviceURL)
                return
            }

            if (response.status === 401) {
                logout()
            }
        })
        .catch(function (error) {
            ShowFailedSessionAlert(deviceURL)
        })
}

function homeButton(authToken, deviceData) {
    let deviceURL = `/device/${deviceData.Device.udid}/home`

    fetch(deviceURL, {
        method: 'POST',
        headers: {
            'X-Auth-Token': authToken
        }
    })
        .then(response => {
            if (response.status === 404) {
                ShowFailedSessionAlert(deviceURL)
                return
            }
        })
        .catch(() => {
            ShowFailedSessionAlert(deviceURL)
        })
}

function lockButton(authToken, deviceData) {
    let deviceURL = `/device/${deviceData.Device.udid}/lock`

    fetch(deviceURL, {
        method: 'POST',
        headers: {
            'X-Auth-Token': authToken
        }
    })
        .then(response => {
            if (response.status === 404) {
                ShowFailedSessionAlert(deviceURL)
                return
            }
        })
        .catch(() => {
            ShowFailedSessionAlert(deviceURL)
        })
}

function unlockButton(authToken, deviceData) {
    let deviceURL = `/device/${deviceData.Device.udid}/unlock`

    fetch(deviceURL, {
        method: 'POST',
        headers: {
            'X-Auth-Token': authToken
        }
    })
        .then(response => {
            if (response.status === 404) {
                ShowFailedSessionAlert(deviceURL)
                return
            }
        })
        .catch(() => {
            ShowFailedSessionAlert(deviceURL)
        })
}