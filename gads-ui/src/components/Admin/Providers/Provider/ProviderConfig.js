import { Alert, Button, MenuItem, Select, Stack, TextField } from '@mui/material'
import axios from 'axios'
import { useContext, useEffect, useState } from 'react'
import { Auth } from '../../../../contexts/Auth'

export default function ProviderConfig({ isNew, data }) {
    useEffect(() => {
        if (data) {
            setOS(data.os)
            setHostAddress(data.host_address)
            setNickname(data.nickname)
            setPort(data.port)
            setAndroid(data.provide_android)
            setIos(data.provide_ios)
            setUseSeleniumGrid(data.use_selenium_grid)
            setSeleniumGrid(data.selenium_grid)
            setWdaBundleId(data.wda_bundle_id)
            setWdaRepoPath(data.wda_repo_path)
            setSupervisionPassword(data.supervision_password)
            setButtonText('Update')
            setUrlPath('update')
        }
    }, [data])
    // Main
    const [authToken, , logout] = useContext(Auth)
    // OS
    const [os, setOS] = useState('windows')
    const [osDisabled, setOsDisabled] = useState(false)
    // Host address
    const [hostAddress, setHostAddress] = useState('')
    const [hostAddressColor, setHostAddressColor] = useState('')
    // Nickname
    const [nickname, setNickname] = useState('')
    const [nicknameColor, setNicknameColor] = useState('')
    // Port
    const [port, setPort] = useState(0)
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
    // Error
    const [showError, setShowError] = useState(false)
    const [errorText, setErrorText] = useState('')
    const [errorColor, setErrorColor] = useState('error')
    // Button
    const [buttonText, setButtonText] = useState('Add')
    // URL path
    const [urlPath, setUrlPath] = useState('add')

    // On successful provider creation reset the form data
    function resetForm() {
        setOS('windows')
        setHostAddress('')
        setNickname('')
        setPort(0)
        setAndroid(false)
        setIos(false)
        setUseSeleniumGrid(false)
        setSeleniumGrid('')
        setSupervisionPassword('')
        setWdaBundleId('')
        setWdaRepoPath('')
    }

    // On pressing Add/Update
    function handleAddClick() {
        setShowError(false)
        let url = `/admin/providers/${urlPath}`
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
        <Stack direction='column' spacing={2} style={{ backgroundColor: 'white', marginLeft: '10px', marginTop: '10px', borderRadius: '10px', padding: '10px', overflowY: 'scroll' }}>
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
            <Button variant='contained' style={{ width: '100px' }} onClick={handleAddClick}>{buttonText}</Button>
            {showError &&
                <Alert color='error'>{errorText}</Alert>
            }
        </Stack>

    )
}