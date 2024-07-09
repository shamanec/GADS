import { Box, Button, Dialog, DialogActions, DialogContent, DialogContentText, DialogTitle, FormControl, Grid, MenuItem, Stack, TextField, Tooltip } from "@mui/material"
import { useContext, useEffect, useState } from "react"
import { api } from "../../../services/api"
import { Auth } from "../../../contexts/Auth"
import ProviderLogsTable from "./ProviderLogsTable"
import CircularProgress from "@mui/material/CircularProgress";
import CheckIcon from "@mui/icons-material/Check";
import CloseIcon from "@mui/icons-material/Close";
import './NewProvidersAdministration.css'

export default function NewProvidersAdministration() {
    const [providers, setProviders] = useState([])
    const { logout } = useContext(Auth)

    function handleGetProvidersData() {
        let url = `/admin/providers`

        api.get(url)
            .then(response => {
                setProviders(response.data)
            })
            .catch(error => {
                if (error.response) {
                    if (error.response.status === 401) {
                        logout()
                    }
                }
            })
    }

    useEffect(() => {
        handleGetProvidersData()
    }, [])

    return (
        <Stack id='outer-stack' direction='row' spacing={2}>
            <Box id='outer-box'>
                <Grid
                    container
                    spacing={2}
                    margin='10px'
                >
                    <Grid item>
                        <NewProvider handleGetProvidersData={handleGetProvidersData}>
                        </NewProvider>
                    </Grid>
                    {providers.map((provider) => {
                        return (
                            <Grid item>
                                <ExistingProvider
                                    providerData={provider}
                                    handleGetProvidersData={handleGetProvidersData}
                                >
                                </ExistingProvider>
                            </Grid>
                        )
                    })
                    }
                </Grid>
            </Box>
        </Stack>
    )
}

function NewProvider({ handleGetProvidersData }) {
    const [os, setOS] = useState('windows')
    const [nickname, setNickname] = useState('')
    const [hostAddress, setHostAddress] = useState('')
    const [port, setPort] = useState(0)
    const [ios, setIos] = useState(false)
    const [android, setAndroid] = useState(false)
    const [wdaRepoPath, setWdaRepoPath] = useState('')
    const [wdaBundleId, setWdaBundleId] = useState('')
    const [useCustomWda, setUseCustomWda] = useState(false)
    const [useSeleniumGrid, setUseSeleniumGrid] = useState(false)
    const [seleniumGridInstance, setSeleniumGridInstance] = useState('')
    const [loading, setLoading] = useState(false);
    const [addProviderStatus, setAddProviderStatus] = useState(null)

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
            body.use_custom_wda = useCustomWda
        }
        body.use_selenium_grid = useSeleniumGrid
        if (useSeleniumGrid) {
            body.selenium_grid = seleniumGridInstance
        }

        let bodyString = JSON.stringify(body)
        return bodyString
    }

    function handleAddProvider(event) {
        setLoading(true)
        setAddProviderStatus(null)
        event.preventDefault()

        let url = `/admin/providers/add`
        let bodyString = buildPayload()

        api.post(url, bodyString, {})
            .then(() => {
                setAddProviderStatus('success')
                setOS('windows')
                setNickname('')
                setHostAddress('')
                setPort(0)
                setIos(false)
                setAndroid(false)
                setWdaRepoPath('')
                setWdaBundleId('')
                setUseCustomWda(false)
                setUseSeleniumGrid(false)
                setSeleniumGridInstance('')
            })
            .catch(() => {
                setAddProviderStatus('error')
            })
            .finally(() => {
                setTimeout(() => {
                    setLoading(false)
                    handleGetProvidersData()
                    setTimeout(() => {
                        setAddProviderStatus(null)
                    }, 2000)
                }, 1000)
            })
    }

    return (
        <Box className='provider-box'>
            <form onSubmit={handleAddProvider}>
                <Stack spacing={2} className='provider-box-stack'>
                    <Tooltip
                        title='Provider OS'
                        arrow
                        placement='top'
                    >
                        <FormControl fullWidth required>
                            <TextField
                                value={os}
                                onChange={(e) => setOS(e.target.value)}
                                select
                                label='OS'
                                required
                                size='small'
                            >
                                <MenuItem value='windows'>Windows</MenuItem>
                                <MenuItem value='linux'>Linux</MenuItem>
                                <MenuItem value='darwin'>macOS</MenuItem>
                            </TextField>
                        </FormControl>
                    </Tooltip>
                    <Tooltip
                        title='Unique name for the provider'
                        arrow
                        placement='top'
                    >
                        <TextField
                            required
                            label='Nickname'
                            value={nickname}
                            autoComplete='off'
                            size='small'
                            onChange={(event) => setNickname(event.target.value)}
                        />
                    </Tooltip>
                    <Tooltip
                        title='Host local network address, e.g. 192.168.1.6'
                        arrow
                        placement='top'
                    >
                        <TextField
                            required
                            label='Host address'
                            value={hostAddress}
                            autoComplete='off'
                            size='small'
                            onChange={(event) => setHostAddress(event.target.value)}
                        />
                    </Tooltip>
                    <Tooltip
                        title='Port for the provider instance, e.g. 10001'
                        arrow
                        placement='top'
                    >
                        <TextField
                            required
                            label='Port'
                            value={port}
                            autoComplete='off'
                            size='small'
                            onChange={(event) => setPort(Number(event.target.value))}
                        />
                    </Tooltip>
                    <Tooltip
                        title='Should the provider set up iOS devices?'
                        arrow
                        placement='top'
                    >
                        <FormControl fullWidth variant='outlined' required>
                            <TextField
                                variant='outlined'
                                value={ios}
                                onChange={(e) => setIos(e.target.value)}
                                select
                                size='small'
                                label='Provide iOS?'
                                required
                            >
                                <MenuItem value={true}>Yes</MenuItem>
                                <MenuItem value={false}>No</MenuItem>
                            </TextField>
                        </FormControl>
                    </Tooltip>
                    <Tooltip
                        title='Should the provider set up Android devices?'
                        arrow
                        placement='top'
                    >
                        <FormControl fullWidth required>
                            <TextField
                                value={android}
                                onChange={(e) => setAndroid(e.target.value)}
                                select
                                label='Provide Android?'
                                required
                                size='small'
                            >
                                <MenuItem value={true}>Yes</MenuItem>
                                <MenuItem value={false}>No</MenuItem>
                            </TextField>
                        </FormControl>
                    </Tooltip>
                    <Tooltip
                        title='WebDriverAgent bundle identifier, e.g. com.facebook.WebDriverAgentRunner.xctrunner'
                        arrow
                        placement='top'
                    >
                        <TextField
                            required
                            size='small'
                            label='WDA bundle ID'
                            value={wdaBundleId}
                            disabled={!ios}
                            autoComplete='off'
                            onChange={(event) => setWdaBundleId(event.target.value)}
                        />
                    </Tooltip>
                    <Tooltip
                        title='WebDriverAgent repository path on the host from which it will be built with `xcodebuild`, e.g. /Users/shamanec/repos/WebDriverAgent'
                        arrow
                        placement='top'
                    >
                        <TextField
                            required
                            size='small'
                            label='WDA repo path'
                            value={wdaRepoPath}
                            disabled={!ios || (ios && os !== 'darwin')}
                            autoComplete='off'
                            onChange={(event) => setWdaRepoPath(event.target.value)}
                        />
                    </Tooltip>
                    <Tooltip
                        title='Select `Yes` if you are using the custom WebDriverAgent from my repositories. It allows for faster tapping/swiping actions on iOS. If you are using mainstream WDA this will break your interactions!'
                        arrow
                        placement='top'
                    >
                        <FormControl fullWidth required>
                            <TextField
                                size='small'
                                value={useCustomWda}
                                onChange={(e) => setUseCustomWda(e.target.value)}
                                select
                                label='Use custom WDA?'
                                required
                                disabled={!ios}
                            >
                                <MenuItem value={true}>Yes</MenuItem>
                                <MenuItem value={false}>No</MenuItem>
                            </TextField>
                        </FormControl>
                    </Tooltip>
                    <Tooltip
                        title='Select `Yes` if you want the provider to register the devices Appium servers as Selenium Grid nodes. You need to have the Selenium Grid instance running separately from the provider!'
                        arrow
                        placement='top'
                    >
                        <FormControl fullWidth required>
                            <TextField
                                size='small'
                                value={useSeleniumGrid}
                                onChange={(e) => setUseSeleniumGrid(e.target.value)}
                                select
                                label='Use Selenium Grid?'
                                required
                            >
                                <MenuItem value={true}>Yes</MenuItem>
                                <MenuItem value={false}>No</MenuItem>
                            </TextField>
                        </FormControl>
                    </Tooltip>
                    <Tooltip
                        title='Selenium Grid instance address, e.g. http://192.168.1.6:4444'
                        arrow
                        placement='top'
                    >
                        <TextField
                            required
                            size='small'
                            label='Selenium Grid instance'
                            value={seleniumGridInstance}
                            autoComplete='off'
                            disabled={!useSeleniumGrid}
                            onChange={(event) => setSeleniumGridInstance(event.target.value)}
                        />
                    </Tooltip>
                    <Button
                        variant='contained'
                        type='submit'
                        style={{
                            backgroundColor: '#2f3b26',
                            color: '#f4e6cd',
                            fontWeight: 'bold',
                            boxShadow: 'none',
                            height: '40px'
                        }}
                        disabled={loading || addProviderStatus === 'success' || addProviderStatus === 'error'}
                    >
                        {loading ? (
                            <CircularProgress size={25} style={{ color: '#f4e6cd' }} />
                        ) : addProviderStatus === 'success' ? (
                            <CheckIcon size={25} style={{ color: '#f4e6cd', stroke: '#f4e6cd', strokeWidth: 2 }} />
                        ) : addProviderStatus === 'error' ? (
                            <CloseIcon size={25} style={{ color: 'red', stroke: 'red', strokeWidth: 2 }} />
                        ) : (
                            'Add provider'
                        )}
                    </Button>
                    <div>All updates to existing provider config require provider instance restart</div>
                </Stack>
            </form>
        </Box>
    )
}

function ExistingProvider({ providerData, handleGetProvidersData }) {
    const [os, setOS] = useState(providerData.os)
    const [nickname, setNickname] = useState(providerData.nickname)
    const [hostAddress, setHostAddress] = useState(providerData.host_address)
    const [port, setPort] = useState(providerData.port)
    const [ios, setIos] = useState(providerData.provide_ios)
    const [android, setAndroid] = useState(providerData.provide_android)
    const [wdaRepoPath, setWdaRepoPath] = useState(providerData.wda_repo_path)
    const [wdaBundleId, setWdaBundleId] = useState(providerData.wda_bundle_id)
    const [useCustomWda, setUseCustomWda] = useState(providerData.use_custom_wda)
    const [useSeleniumGrid, setUseSeleniumGrid] = useState(providerData.use_selenium_grid)
    const [seleniumGridInstance, setSeleniumGridInstance] = useState(providerData.selenium_grid)

    const [openAlert, setOpenAlert] = useState(false)
    const [openLogsDialog, setOpenLogsDialog] = useState(false)

    const [loading, setLoading] = useState(false);
    const [updateProviderStatus, setUpdateProviderStatus] = useState(null)

    function handleDeleteProvider(event) {
        event.preventDefault()

        let url = `/admin/providers/${nickname}`

        api.delete(url)
            .catch(e => {
            })
            .finally(() => {
                handleGetProvidersData()
                setOpenAlert(false)
            })
    }

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
            body.use_custom_wda = useCustomWda
        }
        body.use_selenium_grid = useSeleniumGrid
        if (useSeleniumGrid) {
            body.selenium_grid = seleniumGridInstance
        }

        let bodyString = JSON.stringify(body)
        return bodyString
    }

    function handleUpdateProvider(event) {
        setLoading(true)
        setUpdateProviderStatus(null)
        event.preventDefault()

        let url = `/admin/providers/update`
        let bodyString = buildPayload()

        api.post(url, bodyString, {})
            .then(() => {
                setUpdateProviderStatus('success')
            })
            .catch(() => {
                setUpdateProviderStatus('error')
            })
            .finally(() => {
                setTimeout(() => {
                    setLoading(false)
                    handleGetProvidersData()
                    setTimeout(() => {
                        setUpdateProviderStatus(null)
                    }, 2000)
                }, 1000)
            })
    }

    return (
        <Box className='provider-box'>
            <form onSubmit={handleUpdateProvider}>
                <Stack spacing={2} className='provider-box-stack'>
                    <Tooltip
                        title='Provider OS'
                        arrow
                        placement='top'
                    >
                        <FormControl fullWidth required>
                            <TextField
                                disabled
                                variant='outlined'
                                value={os}
                                onChange={(e) => setOS(e.target.value)}
                                select
                                label='OS'
                                required
                                size='small'
                            >
                                <MenuItem value='windows'>Windows</MenuItem>
                                <MenuItem value='linux'>Linux</MenuItem>
                                <MenuItem value='darwin'>macOS</MenuItem>
                            </TextField>
                        </FormControl>
                    </Tooltip>
                    <Tooltip
                        title='Unique name for the provider'
                        arrow
                        placement='top'
                    >
                        <TextField
                            required
                            label='Nickname'
                            value={nickname}
                            autoComplete='off'
                            size='small'
                            onChange={(event) => setNickname(event.target.value)}
                        />
                    </Tooltip>
                    <Tooltip
                        title='Host local network address, e.g. 192.168.1.6'
                        arrow
                        placement='top'
                    >
                        <TextField
                            required
                            label='Host address'
                            value={hostAddress}
                            autoComplete='off'
                            size='small'
                            onChange={(event) => setHostAddress(event.target.value)}
                        />
                    </Tooltip>
                    <Tooltip
                        title='Port for the provider instance, e.g. 10001'
                        arrow
                        placement='top'
                    >
                        <TextField
                            required
                            label='Port'
                            value={port}
                            autoComplete='off'
                            size='small'
                            onChange={(event) => setPort(Number(event.target.value))}
                        />
                    </Tooltip>
                    <Tooltip
                        title='Should the provider set up iOS devices?'
                        arrow
                        placement='top'
                    >
                        <FormControl fullWidth variant='outlined' required>
                            <TextField
                                variant='outlined'
                                value={ios}
                                onChange={(e) => setIos(e.target.value)}
                                select
                                size='small'
                                label='Provide iOS?'
                                required
                            >
                                <MenuItem value={true}>Yes</MenuItem>
                                <MenuItem value={false}>No</MenuItem>
                            </TextField>
                        </FormControl>
                    </Tooltip>
                    <Tooltip
                        title='Should the provider set up Android devices?'
                        arrow
                        placement='top'
                    >
                        <FormControl fullWidth required>
                            <TextField
                                value={android}
                                onChange={(e) => setAndroid(e.target.value)}
                                select
                                label='Provide Android?'
                                required
                                size='small'
                            >
                                <MenuItem value={true}>Yes</MenuItem>
                                <MenuItem value={false}>No</MenuItem>
                            </TextField>
                        </FormControl>
                    </Tooltip>
                    <Tooltip
                        title='WebDriverAgent bundle identifier, e.g. com.facebook.WebDriverAgentRunner.xctrunner'
                        arrow
                        placement='top'
                    >
                        <TextField
                            required
                            size='small'
                            label='WDA bundle ID'
                            value={wdaBundleId}
                            disabled={!ios}
                            autoComplete='off'
                            onChange={(event) => setWdaBundleId(event.target.value)}
                        />
                    </Tooltip>
                    <Tooltip
                        title='WebDriverAgent repository path on the host from which it will be built with `xcodebuild`, e.g. /Users/shamanec/repos/WebDriverAgent'
                        arrow
                        placement='top'
                    >
                        <TextField
                            required
                            size='small'
                            label='WDA repo path'
                            value={wdaRepoPath}
                            disabled={!ios || (ios && os !== 'darwin')}
                            autoComplete='off'
                            onChange={(event) => setWdaRepoPath(event.target.value)}
                        />
                    </Tooltip>
                    <Tooltip
                        title='Select `Yes` if you are using the custom WebDriverAgent from my repositories. It allows for faster tapping/swiping actions on iOS. If you are using mainstream WDA this will break your interactions!'
                        arrow
                        placement='top'
                    >
                        <FormControl fullWidth required>
                            <TextField
                                size='small'
                                value={useCustomWda}
                                onChange={(e) => setUseCustomWda(e.target.value)}
                                select
                                label='Use custom WDA?'
                                required
                                disabled={!ios}
                            >
                                <MenuItem value={true}>Yes</MenuItem>
                                <MenuItem value={false}>No</MenuItem>
                            </TextField>
                        </FormControl>
                    </Tooltip>
                    <Tooltip
                        title='Select `Yes` if you want the provider to register the devices Appium servers as Selenium Grid nodes. You need to have the Selenium Grid instance running separately from the provider!'
                        arrow
                        placement='top'
                    >
                        <FormControl fullWidth required>
                            <TextField
                                size='small'
                                value={useSeleniumGrid}
                                onChange={(e) => setUseSeleniumGrid(e.target.value)}
                                select
                                label='Use Selenium Grid?'
                                required
                            >
                                <MenuItem value={true}>Yes</MenuItem>
                                <MenuItem value={false}>No</MenuItem>
                            </TextField>
                        </FormControl>
                    </Tooltip>
                    <Tooltip
                        title='Selenium Grid instance address, e.g. http://192.168.1.6:4444'
                        arrow
                        placement='top'
                    >
                        <TextField
                            required
                            size='small'
                            label='Selenium Grid instance'
                            value={seleniumGridInstance}
                            autoComplete='off'
                            disabled={!useSeleniumGrid}
                            onChange={(event) => setSeleniumGridInstance(event.target.value)}
                        />
                    </Tooltip>
                    <Button
                        variant='contained'
                        type='submit'
                        style={{
                            backgroundColor: '#2f3b26',
                            color: '#f4e6cd',
                            fontWeight: 'bold',
                            boxShadow: 'none',
                            height: '40px'
                        }}
                        disabled={loading || updateProviderStatus === 'success' || updateProviderStatus === 'error'}
                    >
                        {loading ? (
                            <CircularProgress size={25} style={{ color: '#f4e6cd' }} />
                        ) : updateProviderStatus === 'success' ? (
                            <CheckIcon size={25} style={{ color: '#f4e6cd', stroke: '#f4e6cd', strokeWidth: 2 }} />
                        ) : updateProviderStatus === 'error' ? (
                            <CloseIcon size={25} style={{ color: 'red', stroke: 'red', strokeWidth: 2 }} />
                        ) : (
                            'Update provider'
                        )}
                    </Button>
                    <Button
                        variant='contained'
                        onClick={() => setOpenLogsDialog(true)}
                        style={{
                            backgroundColor: '#2f3b26',
                            color: '#f4e6cd',
                            fontWeight: 'bold',
                            boxShadow: 'none',
                            height: '40px'
                        }}
                    >Show logs</Button>
                    <Button
                        onClick={() => setOpenAlert(true)}
                        style={{
                            backgroundColor: 'orange',
                            color: '#2f3b26',
                            fontWeight: 'bold',
                            boxShadow: 'none',
                            height: '40px'
                        }}
                    >Delete provider</Button>
                    <Dialog
                        open={openAlert}
                        onClose={() => setOpenAlert(false)}
                    >
                        <DialogTitle>
                            Delete provider from DB?
                        </DialogTitle>
                        <DialogContent>
                            <DialogContentText>
                                Nickname: {nickname}. Host address: {hostAddress}.
                            </DialogContentText>
                        </DialogContent>
                        <DialogActions>
                            <Button onClick={() => setOpenAlert(false)}>Cancel</Button>
                            <Button onClick={handleDeleteProvider} autoFocus>
                                Confirm
                            </Button>
                        </DialogActions>
                    </Dialog>
                    <Dialog
                        fullWidth
                        maxWidth='xl'
                        open={openLogsDialog}
                        onClose={() => setOpenLogsDialog(false)}
                    >
                        <DialogContent id='dialog-content' style={{ overflow: 'hidden', height: '450px' }}>
                            <ProviderLogsTable
                                nickname={nickname}
                            ></ProviderLogsTable>
                        </DialogContent>

                    </Dialog>
                </Stack>
            </form>
        </Box>
    )

}

