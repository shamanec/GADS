import { Badge, Box, Button, CircularProgress, Divider, Grid2, Stack, Tooltip } from "@mui/material";
import { useEffect, useState } from "react";
import { api } from "../../../services/api";
import { useDialog } from "../../../contexts/DialogContext";
import { useSnackbar } from "../../../contexts/SnackBarContext";
import styled from "@emotion/styled";

export default function Config() {
    const [seleniumJarExists, setSeleniumJarExists] = useState(false)
    const [supervisionFileExists, setSupervisionFileExists] = useState(false)
    const [webDriverAgentFileExists, setWebDriverAgentFileExists] = useState(false)
    const [signingPemFileExists, setSigningPemFileExists] = useState(true)
    const [mobileProvisionFileExists, setMobileProvisionFileExists] = useState(false)
    const [androidStreamFileExists, setAndroidStreamFileExists] = useState(false)

    const { showSnackbar } = useSnackbar()
    const { showDialog, hideDialog } = useDialog()

    function handleGetFileData() {
        let url = `/admin/files`

        api.get(url)
            .then(response => {
                let files = response.data
                if (files.length !== 0) {
                    for (const file of files) {
                        if (file.name === 'selenium.jar') {
                            setSeleniumJarExists(true)
                        }
                        if (file.name === 'supervision.p12') {
                            setSupervisionFileExists(true)
                        }
                        if (file.name === 'WebDriverAgent.ipa') {
                            setWebDriverAgentFileExists(true)
                        }
                        if (file.name === 'private_key.pem') {
                            setSigningPemFileExists(true)
                        }
                        if (file.name === 'profile.mobileprovision') {
                            setMobileProvisionFileExists(true)
                        }
                        if (file.name === 'gads-stream.apk') {
                            setAndroidStreamFileExists(true)
                        }
                    }
                }
            })
            .catch(() => {
            })
    }

    useEffect(() => {
        handleGetFileData()
    }, [])

    const showCustomSnackbarError = (message) => {
        showSnackbar({
            message: message,
            severity: 'error',
            duration: 3000,
        })
    }

    const showCustomSnackbarSuccess = (message) => {
        showSnackbar({
            message: message,
            severity: 'success',
            duration: 3000,
        })
    }

    const StyledBox = styled(Box)(() => ({
        borderRadius: '10px',
        padding: '20px',
        display: 'flex',
        flexDirection: 'column',
        backgroundColor: '#9ba984',
        width: '300px',
        height: '300px',
        alignItems: 'center',
        border: '1px solid #ddd',
    }))

    const StyledButton = styled(Button)(() => ({
        backgroundColor: '#2f3b26',
        color: '#f4e6cd',
        fontWeight: 'bold',
        boxShadow: 'none',
        height: '40px',
        width: '100px',
        '&:hover': {
            backgroundColor: '#2f3b26',
            boxShadow: 'none',
        },
    }))

    const StyledLoadingButton = ({ loading, children, ...props }) => {
        return (
            <StyledButton {...props}>
                {loading ? (
                    <CircularProgress size={25} sx={{ color: '#f4e6cd' }} />
                ) : (
                    children
                )}
            </StyledButton>
        )
    }

    return (
        <Grid2 container spacing={2} margin='20px'>
            <Grid2 item>
                <AndroidStreamBox></AndroidStreamBox>
            </Grid2>
            <Grid2 item>
                <WebDriverAgentBox></WebDriverAgentBox>
            </Grid2>
            <Grid2 item>
                <SeleniumJarBox></SeleniumJarBox>
            </Grid2>
            <Grid2 item>
                <SigningPrivateKeyBox></SigningPrivateKeyBox>
            </Grid2>
            <Grid2 item>
                <CSRBox></CSRBox>
            </Grid2>
            <Grid2 item>
                <SigningP12FileBox></SigningP12FileBox>
            </Grid2>
        </Grid2>
    )

    function AndroidStreamBox() {
        const [updatingApk, setUpdatingApk] = useState(false)

        const handleAndroidStreamUpdate = () => {
            let url = `/admin/files/update-android-stream-apk`
            setUpdatingApk(true)

            api.post(url)
                .then(() => {
                    showCustomSnackbarSuccess('Successfully updated GADS Android stream apk!')
                })
                .catch(() => {
                    showCustomSnackbarError('Failed to update GADS Android stream apk!')
                })
                .finally(() => {
                    setTimeout(() => {
                        setUpdatingApk(false)
                    }, 1000)
                })
        }

        return (
            <StyledBox>
                <h3>GADS Android stream</h3>
                <p
                    style={{
                        marginTop: '5px',
                        textAlign: 'center',
                    }}
                >
                    For remote control of Android devices you need GADS-Android-stream apk for the MJPEG stream. {' '}
                    <a
                        href="https://github.com/shamanec/GADS-Android-stream"
                        target="_blank"
                        rel="noopener noreferrer"
                        style={{ color: 'blue', textDecoration: 'underline' }}
                    >
                        Link
                    </a>{' '}
                </p>
                <Stack spacing={1}>
                    <Tooltip
                        arrow
                        placement='left'
                        title='Download the latest GADS Android stream apk from releases and update it in the DB'
                    >
                        <span style={{ display: 'inline-block' }}>
                            <StyledLoadingButton
                                onClick={handleAndroidStreamUpdate}
                                disabled={updatingApk}
                                loading={updatingApk}
                            >{androidStreamFileExists ? 'Update' : 'Get'}</StyledLoadingButton>
                        </span>
                    </Tooltip>
                </Stack>
            </StyledBox>
        )
    }

    function WebDriverAgentBox() {
        return (
            <StyledBox>
                <h3>WebDriverAgent - real devices</h3>
                <p
                    style={{
                        marginTop: '5px',
                        textAlign: 'center',
                    }}
                >To work with real iOS devices you have to upload a prebuilt WebDriverAgent bundled into `.ipa`. You can find instructions {' '}
                    <a
                        href="https://github.com/shamanec/GADS/blob/main/docs/provider.md#prepare-webdriveragent---read-the-full-paragraph"
                        target="_blank"
                        rel="noopener noreferrer"
                        style={{ color: 'blue', textDecoration: 'underline' }}
                    >
                        here
                    </a>{' '}</p>
                <Stack
                    direction='column'
                    spacing={1}
                >
                    <Tooltip
                        arrow
                        placement='left'
                        title='Select the WebDriverAgent ipa file from the file explorer'
                    >
                        <span style={{ display: 'inline-block' }}>
                            <StyledLoadingButton
                                component='label'
                            >
                                <input
                                    type="file"
                                    accept=".ipa"
                                    hidden
                                    onChange={(event) => console.log("OPALE")}
                                />
                                Upload</StyledLoadingButton>
                        </span>
                    </Tooltip>

                </Stack>

            </StyledBox>
        )
    }

    function CSRBox() {
        return (
            <StyledBox>
                <h3>iOS CSR</h3>
                <p
                    style={{
                        marginTop: '5px',
                        textAlign: 'center',
                    }}
                >Apple requires CSR(Certificate Signing Request) to create new certificates on Apple developer portal. You can generate one via GADS if you do not own a macOS machine. Private key is required!</p>
                <Stack
                    direction='column'
                    spacing={1}
                >
                    <StyledButton>Generate</StyledButton>
                    <StyledButton>Download</StyledButton>
                </Stack>
            </StyledBox>
        )
    }

    function SeleniumJarBox() {
        return (
            <StyledBox>
                <h3>Selenium jar</h3>
                <p
                    style={{
                        marginTop: '5px',
                        textAlign: 'center',
                    }}
                >If you want to connect provider Appium nodes to Selenium Grid instance you need to upload a valid Selenium jar. Version 4.13 is recommended.</p>
                <StyledButton>Upload</StyledButton>
            </StyledBox>
        )
    }

    function SigningP12FileBox() {
        return (
            <StyledBox>
                <h3>iOS p12 signing file</h3>
                <p
                    style={{
                        marginTop: '5px',
                        textAlign: 'center',
                    }}
                >If you want to resign WebDriverAgent via GADS (through zsign) you can supply a `.p12` certificate file. You can also generate one if you supply private signing key and developer certificate</p>
                <Stack
                    direction='column'
                    spacing={1}
                >
                    <StyledButton>Upload</StyledButton>
                    <StyledButton>Download</StyledButton>
                    <StyledButton>Generate</StyledButton>
                </Stack>

            </StyledBox>
        )
    }

    function SigningPrivateKeyBox() {


        const handleGeneratePrivateKeyClick = () => {
            if (signingPemFileExists) {
                showDialog('generatePrivateKeyAlert', {
                    title: 'Generate private key pem file?',
                    content: `Private key pem file already exists in DB.`,
                    actions: [
                        { label: 'Cancel', onClick: () => hideDialog() },
                        { label: 'Confirm', onClick: () => handleGeneratePrivateKey() }
                    ],
                    isCloseable: false
                })
            } else {
                handleGeneratePrivateKey()
            }
        }

        const handleGeneratePrivateKey = () => {

        }

        const handleDownloadPrivateKey = () => {

        }

        const handleUploadPrivateKey = () => {

        }

        return (
            <StyledBox>
                <h3>iOS signing private key</h3>
                <p
                    style={{
                        marginTop: '5px',
                        textAlign: 'center',
                    }}
                >If you want to resign WebDriverAgent via GADS you have to upload/generate a private key `.pem` file</p>
                <Stack
                    direction='column'
                    spacing={1}
                >
                    <StyledButton>{signingPemFileExists ? "Update" : "Upload"}</StyledButton>
                    <StyledButton>Download</StyledButton>
                    <Tooltip
                        title={<div>Generate a new private key that can be used for creating CSR(certificate signing request) and re-signing of WebDriverAgent<br />!!! Note that this will replace your currently existing private key file</div>}
                        arrow
                        placement='bottom'
                    >

                        <StyledButton
                            onClick={handleGeneratePrivateKeyClick}
                        >Generate</StyledButton>
                    </Tooltip>
                </Stack>
            </StyledBox>
        )
    }
}