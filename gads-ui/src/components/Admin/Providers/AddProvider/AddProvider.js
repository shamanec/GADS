import { Alert, Box, Button, Grid, MenuItem, Select, Stack, TextField } from "@mui/material"
import axios from "axios"
import { useState } from "react"

export default function AddProvider() {
    // OS
    const [os, setOS] = useState('windows')
    // Host address
    const [hostAddress, setHostAddress] = useState('')
    const [hostAddressColor, setHostAddressColor] = useState('')
    function validateHostAddress(val) {

    }
    // Nickname
    const [nickname, setNickname] = useState('')
    const [nicknameColor, setNicknameColor] = useState('')
    // Port
    const [port, setPort] = useState('')
    const [portColor, setPortColor] = useState('')
    function validatePort(val) {

    }
    // Provide Android
    const [android, setAndroid] = useState(false)
    // Provide iOS
    const [ios, setIos] = useState(false)
    // Use Selenium Grid
    const [useSeleniumGrid, setUseSeleniumGrid] = useState(false)
    // Selenium Grid
    const [seleniumGrid, setSeleniumGrid] = useState('')
    // Supervision password
    const [supervisionPassword, setSupervisionPassword] = useState('')
    // WebDriverAgent bundle id
    const [wdaBundleId, setWdaBundleId] = useState('')
    // WebDriverAgent repo path - MacOS
    const [wdaRepoPath, setWdaRepoPath] = useState('')

    function handleAddClick() {
        let url = `/admin/addProvider`
        let body = {}
        body.os = os
        body.host_address = hostAddress
        body.nickname = nickname
        body.port = port
        body.provide_android = android
        body.provide_ios = ios
        if (ios) {
            body.wda_bundle_id = wdaBundleId
            body.wda_repo_path = wdaRepoPath
            body.supervisionPassword = supervisionPassword
        }
        body.use_selenium_grid = useSeleniumGrid
        if (useSeleniumGrid) {
            body.selenium_grid = seleniumGrid
        }

        let bodyString = JSON.stringify(body)

        axios.post(url, bodyString)
    }

    function handleError(msg) {

    }

    return (
        <Stack direction='column' spacing={2} style={{ backgroundColor: 'white', marginLeft: '10px', marginTop: '10px', borderRadius: '10px', padding: '10px' }}>
            <Stack id='top-stack' direction='row' spacing={2} >
                <Stack id='main-info' style={{ width: '250px', alignItems: 'center' }}>
                    <h4>OS</h4>
                    <Select
                        defaultValue='windows'
                        value={os}
                        onChange={(event) => setOS(event.target.value)}
                        style={{ width: '100%' }}
                    >
                        <MenuItem value='windows'>Windows</MenuItem>
                        <MenuItem value='linux'>Linux</MenuItem>
                        <MenuItem value='macos'>MacOS</MenuItem>
                    </Select>
                    <h4>Nickname</h4>
                    <TextField
                        onChange={e => setNickname(e.target.value)}
                        label="Nickname"
                        color={nicknameColor}
                        required
                        id="outlined-required"
                        autoComplete='off'
                        onKeyUp={e => validateHostAddress(e.target.value)}
                        helperText='Unique nickname for the provider'
                        style={{ width: '100%' }}
                    />
                    <h4>Host address</h4>
                    <TextField
                        onChange={e => setHostAddress(e.target.value)}
                        label="Host address"
                        color={hostAddressColor}
                        required
                        id="outlined-required"
                        autoComplete='off'
                        onKeyUp={e => validateHostAddress(e.target.value)}
                        helperText='Local IP address of the provider host without scheme, e.g. 192.168.1.10'
                        style={{ width: '100%' }}
                    />
                    <h4>Port</h4>
                    <TextField
                        onChange={e => setPort(e.target.value)}
                        label="Port"
                        color={hostAddressColor}
                        required
                        id="outlined-required"
                        autoComplete='off'
                        onKeyUp={e => validatePort(e.target.value)}
                        helperText='The port on which you want the provider instance to run'
                        style={{ width: '100%' }}
                    />
                    <h4>Provide Android devices?</h4>
                    <Select
                        defaultValue={false}
                        value={android}
                        onChange={(event) => setAndroid(event.target.value)}
                        style={{ width: '100%' }}
                    >
                        <MenuItem value={true}>Yes</MenuItem>
                        <MenuItem value={false}>No</MenuItem>
                    </Select>
                    <h4>Provide iOS devices?</h4>
                    <Select
                        defaultValue={false}
                        value={ios}
                        onChange={(event) => setIos(event.target.value)}
                        disabled={os === 'windows'}
                        style={{ width: '100%' }}
                    >
                        <MenuItem value={true}>Yes</MenuItem>
                        <MenuItem value={false}>No</MenuItem>
                    </Select>
                </Stack>
                <Stack id='secondary-info' style={{ width: '250px', alignItems: 'center' }}>
                    <h4>WebDriverAgent bundle ID</h4>
                    <TextField
                        onChange={e => setWdaBundleId(e.target.value)}
                        label="WebDriverAgent bundle ID"
                        color={hostAddressColor}
                        required
                        id="outlined-required"
                        autoComplete='off'
                        onKeyUp={e => validateHostAddress(e.target.value)}
                        disabled={!ios || (ios && os === 'macos')}
                        helperText='Bundle ID of the prebuilt WebDriverAgent.ipa, used by `go-ios` to start it'
                    />
                    <h4>WebDriverAgent repo path</h4>
                    <TextField
                        onChange={e => setWdaRepoPath(e.target.value)}
                        label="WebDriverAgent repo path"
                        color={hostAddressColor}
                        required
                        id="outlined-required"
                        autoComplete='off'
                        onKeyUp={e => validateHostAddress(e.target.value)}
                        helperText='Path on the host to the WebDriverAgent repo to build from, e.g. /Users/shamanec/WebDriverAgent-5.8.3'
                        disabled={!ios || (ios && os !== 'macos')}
                    />
                    <h4>Use Selenium Grid?</h4>
                    <Select
                        defaultValue={false}
                        value={useSeleniumGrid}
                        onChange={(event) => setUseSeleniumGrid(event.target.value)}
                        style={{ width: '100%' }}
                    >
                        <MenuItem value={true}>Yes</MenuItem>
                        <MenuItem value={false}>No</MenuItem>
                    </Select>
                    <h4>Selenium Grid address</h4>
                    <TextField
                        onChange={e => setSeleniumGrid(e.target.value)}
                        label="Selenium Grid"
                        color={hostAddressColor}
                        required
                        id="outlined-required"
                        autoComplete='off'
                        onKeyUp={e => validateHostAddress(e.target.value)}
                        helperText='Address of the Selenium Grid instance, e.g. http://192.168.1.28:4444'
                        disabled={!useSeleniumGrid}
                    />

                </Stack>
            </Stack>
            <Button variant='contained' style={{ width: '100px' }} onClick={handleAddClick}>Add</Button>
            <Alert color='error'>Test</Alert>
        </Stack>

    )
}