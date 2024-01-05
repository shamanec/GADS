import { Alert, Box, Button, Grid, MenuItem, Select, Stack, TextField } from '@mui/material'
import axios from 'axios'
import { useContext, useEffect, useState } from 'react'
import { Auth } from '../../../../contexts/Auth'

export default function ProviderConfig({ isNew, data }) {
    console.log('in provider config')
    console.log(data)
    console.log(isNew)
    let os_string = 'windows'
    let host_address_string = ''
    let nickname_string = ''
    let port_value = 0
    let provide_android = false
    let provide_ios = false
    let use_selenium_grid = false
    let selenium_grid = ''
    let wda_bundle_id = ''
    let wda_repo_path = ''
    let button_string = 'Add'
    let url_path = 'add'
    let supervision_password = ''
    if (data) {
        console.log('inside data')
        console.log(data.os)
        os_string = data.os
        host_address_string = data.host_address
        nickname_string = data.nickname
        port_value = data.port
        provide_android = data.provide_android
        provide_ios = data.provide_ios
        use_selenium_grid = data.use_selenium_grid
        selenium_grid = data.selenium_grid
        wda_bundle_id = data.wda_bundle_id
        wda_repo_path = data.wda_repo_path
        supervision_password = data.supervision_password
        button_string = 'Update'
        url_path = 'update'
    }
    // Main
    const [authToken, , logout] = useContext(Auth)
    // OS
    const [os, setOS] = useState(os_string)
    const [osDisabled, setOsDisabled] = useState(false)
    // Host address
    const [hostAddress, setHostAddress] = useState(host_address_string)
    const [hostAddressColor, setHostAddressColor] = useState('')
    // Nickname
    const [nickname, setNickname] = useState(nickname_string)
    const [nicknameColor, setNicknameColor] = useState('')
    // Port
    const [port, setPort] = useState(port_value)
    const [portColor, setPortColor] = useState('')
    function validatePort(val) {

    }
    // Provide Android
    const [android, setAndroid] = useState(provide_android)
    // Provide iOS
    const [ios, setIos] = useState(provide_ios)
    // Use Selenium Grid
    const [useSeleniumGrid, setUseSeleniumGrid] = useState(use_selenium_grid)
    // Selenium Grid
    const [seleniumGrid, setSeleniumGrid] = useState(selenium_grid)
    // Supervision password
    const [supervisionPassword, setSupervisionPassword] = useState(supervision_password)
    // WebDriverAgent bundle id
    const [wdaBundleId, setWdaBundleId] = useState(wda_bundle_id)
    // WebDriverAgent repo path - MacOS
    const [wdaRepoPath, setWdaRepoPath] = useState(wda_repo_path)
    // Error
    const [showError, setShowError] = useState(false)
    const [errorText, setErrorText] = useState('')
    const [errorColor, setErrorColor] = useState('error')

    // On successful provider creation reset the form data
    function resetForm() {
        setOS(os_string)
        setHostAddress(host_address_string)
        setNickname(nickname_string)
        setPort(port_value)
        setAndroid(provide_android)
        setIos(provide_ios)
        setUseSeleniumGrid(use_selenium_grid)
        setSeleniumGrid(selenium_grid)
        setSupervisionPassword(supervision_password)
        setWdaBundleId(wda_bundle_id)
        setWdaRepoPath(wda_repo_path)
    }

    // On pressing Add/Update
    function handleAddClick() {
        setShowError(false)
        let url = `/admin/providers/${url_path}`
        let bodyString = buildPayload()

        axios.post(url, bodyString, {
            headers: {
                'X-Auth-Token': authToken
            }
        })
            .then(() => {
                resetForm()
            })
            .catch((error) => {
                if (error.response) {
                    if (error.response.status === 401) {
                        logout()
                        return
                    }
                    handleError(error.response.data.error)
                    return
                }
                handleError('Failure')
            })
    }

    // Create the payload for adding/updating provider request
    function buildPayload() {
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
            body.supervision_password = supervisionPassword
        }
        body.use_selenium_grid = useSeleniumGrid
        if (useSeleniumGrid) {
            body.selenium_grid = seleniumGrid
        }

        let bodyString = JSON.stringify(body)
        return bodyString
    }

    function handleError(msg) {
        setErrorText(msg)
        setShowError(true)
    }

    return (
        <Stack direction='column' spacing={2} style={{ backgroundColor: 'white', marginLeft: '10px', marginTop: '10px', borderRadius: '10px', padding: '10px', overflow: 'scroll' }}>
            <Stack id='top-stack' direction='row' spacing={2} >
                <Stack id='main-info' style={{ width: '250px', alignItems: 'center' }}>
                    <h4>OS</h4>
                    <Select
                        defaultValue='windows'
                        value={os}
                        onChange={(event) => setOS(event.target.value)}
                        style={{ width: '100%' }}
                        disabled={!isNew}
                    >
                        <MenuItem value='windows'>Windows</MenuItem>
                        <MenuItem value='linux'>Linux</MenuItem>
                        <MenuItem value='darwin'>MacOS</MenuItem>
                    </Select>
                    <h4>Nickname</h4>
                    <TextField
                        onChange={e => setNickname(e.target.value)}
                        label='Nickname'
                        color={nicknameColor}
                        required
                        id='outlined-required'
                        autoComplete='off'
                        helperText='Unique nickname for the provider'
                        style={{ width: '100%' }}
                        value={nickname}
                        disabled={!isNew}
                    />
                    <h4>Host address</h4>
                    <TextField
                        onChange={e => setHostAddress(e.target.value)}
                        label='Host address'
                        color={hostAddressColor}
                        required
                        id='outlined-required'
                        autoComplete='off'
                        helperText='Local IP address of the provider host without scheme, e.g. 192.168.1.10'
                        style={{ width: '100%' }}
                        value={hostAddress}
                    />
                    <h4>Port</h4>
                    <TextField
                        onChange={e => setPort(Number(e.target.value))}
                        label='Port'
                        color={hostAddressColor}
                        required
                        id='outlined-required'
                        autoComplete='off'
                        onKeyUp={e => validatePort(e.target.value)}
                        helperText='The port on which you want the provider instance to run'
                        style={{ width: '100%' }}
                        value={port}
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
                        label='WebDriverAgent bundle ID'
                        color={hostAddressColor}
                        required
                        id='outlined-required'
                        autoComplete='off'
                        disabled={!ios}
                        helperText='Bundle ID of the prebuilt WebDriverAgent.ipa, used by `go-ios` to start it'
                        value={wdaBundleId}
                    />
                    <h4>WebDriverAgent repo path</h4>
                    <TextField
                        onChange={e => setWdaRepoPath(e.target.value)}
                        label='WebDriverAgent repo path'
                        color={hostAddressColor}
                        required
                        id='outlined-required'
                        autoComplete='off'
                        helperText='Path on the host to the WebDriverAgent repo to build from, e.g. /Users/shamanec/WebDriverAgent-5.8.3'
                        disabled={!ios || (ios && os !== 'darwin')}
                        value={wdaRepoPath}
                    />
                    <h4>Supervision password</h4>
                    <TextField
                        onChange={e => setSupervisionPassword(e.target.value)}
                        label='Supervision password'
                        color={hostAddressColor}
                        id='outlined-required'
                        autoComplete='off'
                        helperText='Password for the supervision profile for iOS devices(leave empty if devices not supervised)'
                        disabled={!ios}
                        value={supervisionPassword}
                    />
                    <h4>Use Selenium Grid?</h4>
                    <Select
                        // defaultValue={useSeleniumGrid}
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
                        label='Selenium Grid'
                        color={hostAddressColor}
                        required
                        id='outlined-required'
                        autoComplete='off'
                        helperText='Address of the Selenium Grid instance, e.g. http://192.168.1.28:4444'
                        disabled={!useSeleniumGrid}
                        value={seleniumGrid}
                    />

                </Stack>
            </Stack>
            <Button variant='contained' style={{ width: '100px' }} onClick={handleAddClick}>{button_string}</Button>
            {showError &&
                <Alert color='error'>{errorText}</Alert>
            }
        </Stack>

    )
}