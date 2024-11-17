import { useEffect, useState } from 'react'
import './StreamCanvas.css'
import { Button, Divider, Grid, Tooltip } from '@mui/material'
import HomeIcon from '@mui/icons-material/Home'
import LockOpenIcon from '@mui/icons-material/LockOpen'
import LockIcon from '@mui/icons-material/Lock'
import KeyboardArrowLeftIcon from '@mui/icons-material/KeyboardArrowLeft'
import KeyboardArrowRightIcon from '@mui/icons-material/KeyboardArrowRight'
import KeyboardArrowUpIcon from '@mui/icons-material/KeyboardArrowUp'
import KeyboardArrowDownIcon from '@mui/icons-material/KeyboardArrowDown'
import { api } from '../../../services/api.js'
import StreamSettings from './StreamSettings.js'
import { useSnackbar } from '../../../contexts/SnackBarContext.js'

export default function StreamCanvas({ deviceData, shouldShowStream }) {
    const { showSnackbar } = useSnackbar()
    const [isPortrait, setIsPortrait] = useState(true)
    const [canvasDimensions, setCanvasDimensions] = useState({
        width: 0,
        height: 0
    })

    let deviceX = parseInt(deviceData.screen_width, 10)
    let deviceY = parseInt(deviceData.screen_height, 10)
    let deviceScreenRatio = deviceX / deviceY
    let deviceLandscapeScreenRatio = deviceY / deviceX
    let deviceOS = deviceData.os
    let usesCustomWda = deviceData.usesCustomWda
    let udid = deviceData.udid

    let streamUrl = ''
    if (deviceData.os === 'ios') {
        streamUrl = `http://192.168.1.41:10000/device/${deviceData.udid}/ios-stream-mjpeg`
        // streamUrl = `/device/${deviceData.udid}/ios-stream-mjpeg`
    } else {
        streamUrl = `http://192.168.1.41:10000/device/${deviceData.udid}/android-stream-mjpeg`
        // streamUrl = `/device/${deviceData.udid}/android-stream-mjpeg`
    }

    const handleOrientationButtonClick = (isPortrait) => {
        setIsPortrait(isPortrait)
    }

    useEffect(() => {
        const updateCanvasDimensions = () => {
            let calculatedWidth, calculatedHeight
            if (isPortrait) {
                calculatedHeight = window.innerHeight * 0.7
                calculatedWidth = calculatedHeight * deviceScreenRatio
            } else {
                calculatedWidth = window.innerWidth * 0.4
                calculatedHeight = calculatedWidth / deviceLandscapeScreenRatio
            }

            setCanvasDimensions({
                width: calculatedWidth,
                height: calculatedHeight
            })
        }

        const imgElement = document.getElementById('image-stream')

        // Temporarily remove the stream source
        imgElement.src = ''

        updateCanvasDimensions()

        // Reapply the stream URL after the resize is complete
        imgElement.src = shouldShowStream ? streamUrl : ''

        // Set resize listener
        window.addEventListener('resize', updateCanvasDimensions)

        return () => {
            window.stop()
            window.removeEventListener('resize', updateCanvasDimensions)
        }
    }, [isPortrait])

    const showCustomSnackbarError = (message) => {
        showSnackbar({
            message: message,
            severity: 'error',
            duration: 3000,
        })
    }

    return (
        <Grid
            spacing={1}
            direction='row'
            display='flex'
            justifyContent='center'
            alignItems='flex-start'
        >

            <div
                id='phone-imitation'
            >
                <h3
                    style={{
                        color: '#2f3b26',
                        display: 'flex',
                        fontFamily: 'Verdana',
                        justifyContent: 'center'
                    }}
                >{deviceData.model}</h3>
                <div
                    id='stream-div'
                    style={{
                        width: canvasDimensions.width,
                        height: canvasDimensions.height
                    }}
                >
                    <Canvas></Canvas>
                    <Stream></Stream>
                </div>
                <Grid
                    height='30px'
                    display='flex'
                    justifyContent='center'
                >
                </Grid>
            </div >

            <Grid
                direction='column'
                width='150px'
                marginLeft='10px'
                spacing={1}
                container
            >
                <Grid item>
                    <Tooltip
                        title='This does not change the orientation of the device itself, just updates the UI if the device orientation is already changed'
                        arrow
                        placement='top'
                    >
                        <Button
                            variant={'contained'}
                            color={'secondary'}
                            onClick={() => handleOrientationButtonClick(true)}
                            disabled={isPortrait}
                            sx={{ width: '100%' }}
                        >
                            Portrait
                        </Button>
                    </Tooltip>
                </Grid>
                <Grid item>
                    <Tooltip
                        title='This does not change the orientation of the device itself, just updates the UI if the device orientation is already changed'
                        arrow
                        placement='top'
                    >
                        <Button
                            variant={'contained'}
                            color={'secondary'}
                            onClick={() => handleOrientationButtonClick(false)}
                            disabled={!isPortrait}
                            sx={{ width: '100%' }}
                        >
                            Landscape
                        </Button>
                    </Tooltip>
                </Grid>
                <Grid item>
                    <Divider></Divider>
                </Grid>
                <Grid item>
                    <Button
                        onClick={() => homeButton()}
                        className='canvas-buttons'
                        startIcon={<HomeIcon />}
                        variant='contained'
                        style={{
                            fontWeight: 'bold',
                            color: '#9ba984',
                            backgroundColor: '#2f3b26',
                            width: '100%'
                        }}
                    >Home</Button>
                </Grid>
                <Grid item>
                    <Button
                        onClick={() => lockButton()}
                        className='canvas-buttons'
                        startIcon={<LockIcon />}
                        variant='contained'
                        style={{
                            fontWeight: 'bold',
                            color: '#9ba984',
                            backgroundColor: '#2f3b26',
                            width: '100%'
                        }}
                    >Lock</Button>
                </Grid>
                <Grid item>
                    <Button
                        onClick={() => unlockButton()}
                        className='canvas-buttons'
                        startIcon={<LockOpenIcon />}
                        variant='contained'
                        style={{
                            fontWeight: 'bold',
                            color: '#9ba984',
                            backgroundColor: '#2f3b26',
                            width: '100%'
                        }}
                    >Unlock</Button>
                </Grid>
                <Grid item>
                    <Button
                        onClick={() => swipeLeft()}
                        className='canvas-buttons'
                        variant='contained'
                        startIcon={<KeyboardArrowRightIcon />}
                        style={{
                            fontWeight: 'bold',
                            color: '#9ba984',
                            backgroundColor: '#2f3b26',
                            width: '100%'
                        }}
                    >Swipe</Button>
                </Grid>
                <Grid item>
                    <Button
                        onClick={() => swipeRight()}
                        className='canvas-buttons'
                        variant='contained'
                        startIcon={<KeyboardArrowLeftIcon />}
                        style={{
                            fontWeight: 'bold',
                            color: '#9ba984',
                            backgroundColor: '#2f3b26',
                            width: '100%'
                        }}
                    >Swipe</Button>
                </Grid>
                <Grid item>
                    <Button
                        onClick={() => swipeUp()}
                        className='canvas-buttons'
                        variant='contained'
                        startIcon={<KeyboardArrowDownIcon />}
                        style={{
                            fontWeight: 'bold',
                            color: '#9ba984',
                            backgroundColor: '#2f3b26',
                            width: '100%'
                        }}
                    >Swipe</Button>
                </Grid>
                <Grid item>
                    <Button
                        onClick={() => swipeDown()}
                        className='canvas-buttons'
                        variant='contained'
                        startIcon={<KeyboardArrowUpIcon />}
                        style={{
                            fontWeight: 'bold',
                            color: '#9ba984',
                            backgroundColor: '#2f3b26',
                            width: '100%'
                        }}
                    >Swipe</Button>
                </Grid>
                <Grid item>
                    <StreamSettings deviceData={deviceData}></StreamSettings>
                </Grid>
                <Grid item>
                    <Tooltip
                        title='Refresh the Appium session'
                        arrow
                        position='top'
                    >
                        <Button
                            onClick={() => unlockButton()}
                            className='canvas-buttons'
                            startIcon={
                                <img
                                    src="/images/appium-logo.png"
                                    alt="icon"
                                    style={{
                                        width: '24px',
                                        height: '24px',
                                    }}
                                />
                            }
                            variant='contained'
                            style={{
                                fontWeight: 'bold',
                                color: '#9ba984',
                                backgroundColor: '#2f3b26',
                                width: '100%'
                            }}
                        >
                            Refresh
                        </Button>
                    </Tooltip>
                </Grid>
            </Grid>
        </Grid >
    )

    function Canvas() {
        var tapStartAt = 0
        var coord1
        var coord2

        function getCursorCoordinates(event) {
            const rect = event.currentTarget.getBoundingClientRect()
            const x = event.clientX - rect.left
            const y = event.clientY - rect.top
            return [x, y]
        }

        function handleMouseDown(event) {
            tapStartAt = (new Date()).getTime()
            coord1 = getCursorCoordinates(event)
        }

        function handleMouseUp(event) {
            coord2 = getCursorCoordinates(event)
            // get the time of finishing the click on the canvas
            var tapEndAt = (new Date()).getTime()

            console.log('Tap on ' + coord1[0] + 'x' + coord1[1] + ', released on ' + coord2[0] + 'x' + coord2[1])

            var mouseEventsTimeDiff = tapEndAt - tapStartAt

            // if the difference of time between click down and click up is more than 500ms assume it is a swipe, not a tap
            // to allow flick swipes we also check the difference between the gesture coordinates
            // x1, y1 = mousedown coordinates
            // x2, y2 = mouseup coordinates
            // if x2 > x1*1.1 - it is probably a swipe left to right
            // if x2 < x1*0.9 - it is probably a swipe right to left
            // if y2 < y1*0.9 - it is probably a swipe bottom to top
            // if y2 > y1*1.1 - it is probably a swipe top to bottom
            if (mouseEventsTimeDiff > 500 && coord2[0] > coord1[0] * 1.1 || coord2[0] < coord1[0] * 0.9 || coord2[1] < coord1[1] * 0.9 || coord2[1] > coord1[1] * 1.1) {
                swipeCoordinates(coord1, coord2)
            } else if (mouseEventsTimeDiff < 500) {
                tapCoordinates(coord1)
            } else {
                touchAndHoldCoordinates(coord1)
            }
        }

        return (
            <canvas
                id='actions-canvas'
                width={canvasDimensions.width + 'px'}
                height={canvasDimensions.height + 'px'}
                onMouseDown={handleMouseDown}
                onMouseUp={handleMouseUp}
                style={{ position: 'absolute' }}
            ></canvas>
        )
    }

    function Stream() {
        return (
            <img
                id='image-stream'
                width={canvasDimensions.width + 'px'}
                height={canvasDimensions.height + 'px'}
                style={{ display: 'block' }}
                src={shouldShowStream ? streamUrl : ''}
            ></img>
        )
    }

    function swipeUp() {
        let startX = canvasDimensions.width / 2
        let endX = canvasDimensions.width / 2
        let startY = canvasDimensions.height - (canvasDimensions.height * 0.75)
        let endY = canvasDimensions.height - (canvasDimensions.height * 0.25)
        let coord1 = [startX, startY]
        let coord2 = [endX, endY]
        swipeCoordinates(coord1, coord2)
    }

    function swipeDown() {
        let startX = canvasDimensions.width / 2
        let endX = canvasDimensions.width / 2
        let startY = canvasDimensions.height - (canvasDimensions.height * 0.25)
        let endY = canvasDimensions.height - (canvasDimensions.height * 0.75)
        let coord1 = [startX, startY]
        let coord2 = [endX, endY]
        swipeCoordinates(coord1, coord2)
    }

    function swipeLeft() {
        let startX = canvasDimensions.width - (canvasDimensions.width * 0.20)
        let endX = canvasDimensions.width - (canvasDimensions.width * 0.80)
        let startY = canvasDimensions.height / 2
        let endY = canvasDimensions.height / 2
        let coord1 = [startX, startY]
        let coord2 = [endX, endY]
        swipeCoordinates(coord1, coord2)
    }

    function swipeRight() {
        let startX = canvasDimensions.width - (canvasDimensions.width * 0.80)
        let endX = canvasDimensions.width - (canvasDimensions.width * 0.20)
        let startY = canvasDimensions.height / 2
        let endY = canvasDimensions.height / 2
        let coord1 = [startX, startY]
        let coord2 = [endX, endY]
        swipeCoordinates(coord1, coord2)
    }

    // tap using coordinates
    function tapCoordinates(pos) {
        // set initial x and y tap coordinates
        let x = pos[0]
        let y = pos[1]

        let finalX = (x / canvasDimensions.width) * deviceX
        let finalY = (y / canvasDimensions.height) * deviceY
        // If its portrait we keep the x and y as is
        if (!isPortrait) {
            // If its landscape
            // And its Android we still keep the x and y as is because for Android Appium does the coordinates are actually corresponding to the view
            // If we are in portrait X is X and Y is Y and when we are in landscape its the same
            if (deviceOS === 'android') {
                finalX = (x / canvasDimensions.height) * deviceX
                finalY = (y / canvasDimensions.width) * deviceY
            }
            if (deviceOS === 'ios') {
                if (usesCustomWda) {
                    finalX = deviceX - ((y / canvasDimensions.height) * deviceX)
                    finalY = (x / canvasDimensions.width) * deviceY
                } else {
                    // On normal WDA in landscape X corresponds to the canvas width but it is actually the device's height
                    finalX = (x / canvasDimensions.width) * deviceY
                    // Y corresponds to the canvas height but it is actually the device's width
                    finalY = (y / canvasDimensions.height) * deviceX
                }
            }
        }

        let jsonData = JSON.stringify({
            'x': finalX,
            'y': finalY
        })

        let deviceURL = `/device/${udid}`

        api.post(deviceURL + '/tap', jsonData)
            .then(response => {
                if (response.status === 404) {
                    return
                }
            })
            .catch((error) => {
                if (error.response) {
                    if (error.response.status === 404) {
                        showCustomSnackbarError('Tap failed - Appium session has expired!')
                    } else {
                        showCustomSnackbarError('Tap failed!')
                    }
                } else {
                    showCustomSnackbarError('Tap failed!')
                }
            })
    }

    function touchAndHoldCoordinates(pos) {
        // set initial x and y tap coordinates
        let x = pos[0]
        let y = pos[1]

        let finalX = (x / canvasDimensions.width) * deviceX
        let finalY = (y / canvasDimensions.height) * deviceY

        // If its portrait we keep the x and y as is
        if (!isPortrait) {
            // If its landscape
            // And its Android we still keep the x and y as is because for Android Appium does the coordinates are actually corresponding to the view
            // If we are in portrait X is X and Y is Y and when we are in landscape its the same
            if (deviceOS === 'android') {
                finalX = (x / canvasDimensions.height) * deviceX
                finalY = (y / canvasDimensions.width) * deviceY
            }
            if (deviceOS === 'ios') {
                if (usesCustomWda) {
                    finalX = deviceX - ((y / canvasDimensions.height) * deviceX)
                    finalY = (x / canvasDimensions.width) * deviceY
                } else {
                    // On normal WDA in landscape X corresponds to the canvas width but it is actually the device's height
                    finalX = (x / canvasDimensions.width) * deviceY
                    // Y corresponds to the canvas height but it is actually the device's width
                    finalY = (y / canvasDimensions.height) * deviceX
                }
            }
        }

        let jsonData = JSON.stringify({
            'x': finalX,
            'y': finalY
        })

        let deviceURL = `/device/${udid}`

        api.post(deviceURL + '/touchAndHold', jsonData)
            .then(() => { })
            .catch((error) => {
                if (error.response) {
                    if (error.response.status === 404) {
                        showCustomSnackbarError('Touch & hold failed - Appium session has expired!')
                    } else {
                        showCustomSnackbarError('Touch & hold failed!')
                    }
                } else {
                    showCustomSnackbarError('Touch & hold failed!')
                }
            })
    }

    function swipeCoordinates(coord1, coord2) {
        var firstCoordX = coord1[0]
        var firstCoordY = coord1[1]
        var secondCoordX = coord2[0]
        var secondCoordY = coord2[1]

        // Set up the portrait tap coordinates
        // We divide the current coordinate by the canvas size to get the ratio
        // Then we multiply by the actual device width or height to get the actual coordinates on the device that we should send
        let firstXFinal = (firstCoordX / canvasDimensions.width) * deviceX
        let firstYFinal = (firstCoordY / canvasDimensions.height) * deviceY
        let secondXFinal = (secondCoordX / canvasDimensions.width) * deviceX
        let secondYFinal = (secondCoordY / canvasDimensions.height) * deviceY

        // If in landscape we need to do different recalculations
        if (!isPortrait) {
            // If the device is android we just reverse the calculations
            // For X we divide the coordinate by the canvas height to get the correct ratio
            // And for Y we divide the coordinate by the canvas width to get the correct ratio
            // Multiplication is the same as for portrait because Appium for Android follows some actual logic
            if (deviceOS === 'android') {
                firstXFinal = (firstCoordX / canvasDimensions.height) * deviceX
                firstYFinal = (firstCoordY / canvasDimensions.width) * deviceY
                secondXFinal = (secondCoordX / canvasDimensions.height) * deviceX
                secondYFinal = (secondCoordY / canvasDimensions.width) * deviceY
            } else {
                // For iOS its complete sh*t
                if (usesCustomWda) {
                    // NB: All calculations below are when the device is on its right side landscape(your left)
                    // For custom WDA the 0 X coordinate is at the bottom of the canvas
                    // And the 0 Y coordinate is at the left end of the canvas
                    // This means that they kinda follow the portrait logic but inverted based on the side it is in landscape
                    // Imagine a device that is X:Y=100:200
                    // If you swipe vertically from bottom to the middle you are essentially swiping coordinates on the device X:0 and X: 50
                    // And if you swipe horizontally from left to middle you are essentially swiping coordinates on the device Y: 0 and Y: 100
                    // But on the canvas those are Y coordinates
                    // And this is where it gets funky
                    // For X we get the ratio from the canvas Y coordinate and the canvas height
                    // And we multiply it by the device width because that is what it corresponds to
                    // Then we subtract the value from the actual deviceX because the device width starts from the bottom of the canvas height
                    firstXFinal = deviceX - ((firstCoordY / canvasDimensions.height) * deviceX)
                    // For Y we get the ratio from the canvas X coordinate and the canvas width
                    // And we multiply it by the device height because that is what it corresponds to
                    firstYFinal = (firstCoordX / canvasDimensions.width) * deviceY
                    // Same goes for the other two coordinates when the mouse is released
                    secondXFinal = deviceX - ((secondCoordY / canvasDimensions.height) * deviceX)
                    secondYFinal = (secondCoordX / canvasDimensions.width) * deviceY
                } else {
                    // On normal WDA the X coordinates correspond to the device height
                    // and the Y coordinates correspond to the device width
                    // So we calculate ratio based on canvas dimensions and coordinates
                    // But multiply by device height for X swipe coordinates and by device width for Y swipe coordinates
                    firstXFinal = (firstCoordX / canvasDimensions.width) * deviceY
                    firstYFinal = (firstCoordY / canvasDimensions.height) * deviceX
                    secondXFinal = (secondCoordX / canvasDimensions.width) * deviceY
                    secondYFinal = (secondCoordY / canvasDimensions.height) * deviceX
                }
            }
        }
        console.log('Swipe is ' + firstXFinal + ':' + firstYFinal + '-' + secondXFinal + ':' + secondYFinal)

        let jsonData = JSON.stringify({
            'x': firstXFinal,
            'y': firstYFinal,
            'endX': secondXFinal,
            'endY': secondYFinal
        })

        let deviceURL = `/device/${udid}`

        api.post(deviceURL + '/swipe', jsonData)
            .then(() => { })
            .catch((error) => {
                if (error.response) {
                    if (error.response.status === 404) {
                        showCustomSnackbarError('Swipe failed - Appium session has expired!')
                    } else {
                        showCustomSnackbarError('Swipe failed!')
                    }
                } else {
                    showCustomSnackbarError('Swipe failed!')
                }
            })
    }

    function homeButton() {
        let deviceURL = `/device/${deviceData.udid}/home`

        api.post(deviceURL)
            .then(() => { })
            .catch((error) => {
                if (error.response) {
                    if (error.response.status === 404) {
                        showCustomSnackbarError('Navigation to Home failed - Appium session has expired!')
                    } else {
                        showCustomSnackbarError('Navigation to Home failed!')
                    }
                } else {
                    showCustomSnackbarError('Navigation to Home failed!')
                }
            })
    }

    function lockButton() {
        let deviceURL = `/device/${deviceData.udid}/lock`

        api.post(deviceURL)
            .then(() => { })
            .catch((error) => {
                if (error.response) {
                    if (error.response.status === 404) {
                        showCustomSnackbarError('Device lock failed - Appium session has expired!')
                    } else {
                        showCustomSnackbarError('Device lock failed!')
                    }
                } else {
                    showCustomSnackbarError('Device lock failed!')
                }
            })
    }

    function unlockButton() {
        let deviceURL = `/device/${deviceData.udid}/unlock`

        api.post(deviceURL)
            .then(() => { })
            .catch((error) => {
                if (error.response) {
                    if (error.response.status === 404) {
                        showCustomSnackbarError('Device unlock failed - Appium session has expired!')
                    } else {
                        showCustomSnackbarError('Device unlock failed!')
                    }
                } else {
                    showCustomSnackbarError('Device unlock failed!')
                }
            })
    }
}