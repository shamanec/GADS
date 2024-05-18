import { Alert, Button, MenuItem, Select, Stack, TextField } from '@mui/material'
import axios from 'axios'
import { useContext, useEffect, useState } from 'react'
import { Auth } from '../../../../contexts/Auth'

export default function ProviderConfig({ isNew, data, setProviders }) {
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
            setUseCustomWda(data.use_custom_wda)
        }
    }, [data])
    // Main
    const [authToken, , logout] = useContext(Auth)
    // OS
    const [os, setOS] = useState('windows')
    // Host address
    const [hostAddress, setHostAddress] = useState('')
    // Nickname
    const [nickname, setNickname] = useState('')
    // Port
    const [port, setPort] = useState(0)
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
    // Custom WebDriverAgent
    const [useCustomWda, setUseCustomWda] = useState(false)
    // WebDriverAgent bundle id
    const [wdaBundleId, setWdaBundleId] = useState('')
    // WebDriverAgent repo path - MacOS
    const [wdaRepoPath, setWdaRepoPath] = useState('')
    // Error
    const [showError, setShowError] = useState(false)
    const [errorText, setErrorText] = useState('')
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
        setUseCustomWda(false)
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
            .then((response) => {
                if (isNew) {
                    resetForm()
                }
                if (urlPath === 'add') {
                    console.log(' is add')
                    console.log(response.data)
                    setProviders(response.data)
                }
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
            body.use_custom_wda = useCustomWda
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
        <Stack
            direction='column'
            spacing={2}
            style={{
                backgroundColor: '#78866B',
                maxWidth: '500px',
                height: '700px',
                marginLeft: '10px',
                marginTop: '10px',
                borderRadius: '10px',
                padding: '10px',
                overflowY: 'scroll'
            }}
        >
            <Stack
                id='top-stack'
                direction='row'
                spacing={2}
                style={{
                    marginTop: '10px'
                }}
            >
                <Stack
                    spacing={2}
                    id='main-info'
                    style={{
                        alignItems: 'center',
                        width: '50%'
                    }}
                >
                    <Select
                        defaultValue='windows'
                        value={os}
                        onChange={(event) => setOS(event.target.value)}
                        style={{ width: '100%', height: '40px' }}
                        disabled={!isNew}
                    >
                        <MenuItem value='windows'>Windows</MenuItem>
                        <MenuItem value='linux'>Linux</MenuItem>
                        <MenuItem value='darwin'>MacOS</MenuItem>
                    </Select>
                    <TextField
                        onChange={e => setNickname(e.target.value)}
                        label='Nickname'
                        required
                        id='outlined-required'
                        autoComplete='off'
                        helperText='Unique nickname for the provider'
                        fullWidth
                        size='small'
                        value={nickname}
                        disabled={!isNew}
                    />
                    <TextField
                        onChange={e => setHostAddress(e.target.value)}
                        label='Host address'
                        required
                        id='outlined-required'
                        autoComplete='off'
                        helperText='Local IP address of the provider host without scheme, e.g. 192.168.1.10'
                        fullWidth
                        size='small'
                        value={hostAddress}
                        InputLabelProps={{ style: { fontSize: 14 } }}
                    />
                    <TextField
                        onChange={e => setPort(Number(e.target.value))}
                        label='Port'
                        required
                        id='outlined-required'
                        autoComplete='off'
                        helperText='The port on which you want the provider instance to run'
                        fullWidth
                        size='small'
                        value={port}
                    />
                    <div style={{ fontWeight: '500' }}>Provide Android?</div>
                    <Select
                        defaultValue={false}
                        value={android}
                        onChange={(event) => setAndroid(event.target.value)}
                        style={{
                            width: '100%',
                            height: '40px'
                        }}
                    >
                        <MenuItem value={true}>Yes</MenuItem>
                        <MenuItem value={false}>No</MenuItem>
                    </Select>
                    <div style={{ fontWeight: '500' }}>Provide iOS?</div>
                    <Select
                        defaultValue={false}
                        value={ios}
                        onChange={(event) => setIos(event.target.value)}
                        disabled={os === 'windows'}
                        style={{
                            width: '100%',
                            height: '40px'
                        }}
                    >
                        <MenuItem value={true}>Yes</MenuItem>
                        <MenuItem value={false}>No</MenuItem>
                    </Select>
                </Stack>
                <Stack
                    spacing={2}
                    id='secondary-info'
                    style={{
                        width: '50%',
                        alignItems: 'center'
                    }}
                >
                    <TextField
                        onChange={e => setWdaBundleId(e.target.value)}
                        label='WebDriverAgent bundle ID'
                        required
                        id='outlined-required'
                        autoComplete='off'
                        disabled={!ios}
                        helperText='Bundle ID of the prebuilt WebDriverAgent.ipa, used by `go-ios` to start it'
                        value={wdaBundleId}
                        size='small'
                        fullWidth
                    />
                    <TextField
                        onChange={e => setWdaRepoPath(e.target.value)}
                        label='WebDriverAgent repo path'
                        required
                        id='outlined-required'
                        autoComplete='off'
                        helperText='Path on the host to the WebDriverAgent repo to build from, e.g. /Users/shamanec/WebDriverAgent-5.8.3'
                        disabled={!ios || (ios && os !== 'darwin')}
                        value={wdaRepoPath}
                        size='small'
                        fullWidth
                    />
                    <TextField
                        onChange={e => setSupervisionPassword(e.target.value)}
                        label='Supervision password'
                        id='outlined-required'
                        autoComplete='off'
                        helperText='Password for the supervision profile for iOS devices(leave empty if devices not supervised)'
                        disabled={!ios}
                        value={supervisionPassword}
                        size='small'
                        fullWidth
                    />
                    <div style={{fontWeight: '500'}}>Use custom WebDriverAgent?</div>
                    <Select
                        defaultValue={false}
                        value={useCustomWda}
                        onChange={(event) => setUseCustomWda(event.target.value)}
                        style={{
                            width: '100%',
                            height: '40px'
                        }}
                    >
                        <MenuItem value={true}>Yes</MenuItem>
                        <MenuItem value={false}>No</MenuItem>
                    </Select>
                    <div style={{fontWeight: '500'}}>Use Selenium Grid?</div>
                    <Select
                        // defaultValue={useSeleniumGrid}
                        value={useSeleniumGrid}
                        onChange={(event) => setUseSeleniumGrid(event.target.value)}
                        style={{
                            width: '100%',
                            height: '40px'
                        }}
                    >
                        <MenuItem value={true}>Yes</MenuItem>
                        <MenuItem value={false}>No</MenuItem>
                    </Select>
                    <TextField
                        onChange={e => setSeleniumGrid(e.target.value)}
                        label='Selenium Grid'
                        required
                        id='outlined-required'
                        autoComplete='off'
                        helperText='Address of the Selenium Grid instance, e.g. http://192.168.1.28:4444'
                        disabled={!useSeleniumGrid}
                        value={seleniumGrid}
                        size='small'
                        fullWidth
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