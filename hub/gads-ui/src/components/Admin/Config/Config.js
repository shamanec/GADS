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
    const [signingPemFileExists, setSigningPemFileExists] = useState(false)
    const [mobileProvisionFileExists, setMobileProvisionFileExists] = useState(false)
    const [androidStreamFileExists, setAndroidStreamFileExists] = useState(false)
    const [signingCertificateFileExists, setSigningCertificateFileExists] = useState(false)

    const { hideSnackbar, showSnackbar } = useSnackbar()
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
                        if (file.name === 'signing_key.pem') {
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

    const showCustomSnackbarError = ({ message, timeout = 3000 }) => {
        showSnackbar({
            message: message,
            severity: 'error',
            duration: timeout,
        })
    }

    const showCustomSnackbarSuccess = (message, timeout = 3000) => {
        showSnackbar({
            message: message,
            severity: 'success',
            duration: timeout,
        })
    }

    const StyledBox = styled(Box)(() => ({
        borderRadius: '10px',
        padding: '20px',
        display: 'flex',
        flexDirection: 'column',
        backgroundColor: '#9ba984',
        width: '300px',
        height: '320px',
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
        '&:disabled': {
            backgroundColor: '#667a57'
        }
    }))

    const StyledLoadingButton = ({ loading, tooltipText, children, ...props }) => {
        return (
            <Tooltip
                arrow
                placement='left'
                title={tooltipText}
                disableInteractive={true}
            >
                <span style={{ display: 'inline-block' }}>
                    <StyledButton {...props}>
                        {loading ? (
                            <CircularProgress size={25} sx={{ color: '#f4e6cd' }} />
                        ) : (
                            children
                        )}
                    </StyledButton>
                </span>
            </Tooltip>
        )
    }

    const FileInfo = ({ existsText, notExistsText, exists }) => {
        return (
            <p
                style={{
                    color: exists ? 'green' : 'red',
                    fontSize: '15px',
                    fontWeight: 'bold'
                }}
            >
                {exists ? existsText : notExistsText}
            </p>
        )
    }

    const StyledUploadLoadingButton = ({ filename = '', allowedFileExtension, tooltipText, children, ...props }) => {
        const [isUploading, setIsUploading] = useState(false)
        const [inputKey, setInputKey] = useState(Date.now())

        const handleUploadFile = (e) => {
            hideSnackbar()

            if (e.target.files) {
                const targetFile = e.target.files[0]
                if (!targetFile) {
                    return
                }

                const form = new FormData()

                form.append('file', targetFile)
                if (filename !== '') {
                    form.append('fileName', filename)
                } else {
                    form.append('fileName', targetFile.name)
                }
                setIsUploading(true)

                api.post('/admin/upload-config-file', form, {
                    headers: {
                        'Content-Type': 'multipart/form-data',
                    },
                })
                    .then(() => {
                        handleGetFileData()
                        showCustomSnackbarSuccess('File uploaded!')
                    })
                    .catch(() => {
                        showCustomSnackbarError('File upload failed!')
                    })
                    .finally(() => {
                        setInputKey(Date.now())
                        setTimeout(() => {
                            setIsUploading(false)
                        }, 1000)
                    })
            }
        }

        return (
            <StyledLoadingButton
                component='label'
                loading={isUploading}
                tooltipText={tooltipText}
                children={children}
                {...props}
            >
                <input
                    key={inputKey}
                    type="file"
                    accept={allowedFileExtension}
                    hidden
                    onChange={(event) => handleUploadFile(event)}
                />
                Upload
            </StyledLoadingButton>
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
                <IOSSupervisionBox></IOSSupervisionBox>
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
                    For remote control of Android devices you need GADS-Android-stream apk for the MJPEG stream({''}
                    <a
                        href="https://github.com/shamanec/GADS-Android-stream"
                        target="_blank"
                        rel="noopener noreferrer"
                        style={{ color: 'blue', textDecoration: 'underline' }}
                    >
                        link
                    </a>{''}). The button below will update it to the latest release.
                </p>
                <Stack
                    spacing={1}
                    alignItems={"center"}
                >
                    <StyledLoadingButton
                        onClick={handleAndroidStreamUpdate}
                        tooltipText='Automatically update GADS-Android-stream apk'
                        disabled={updatingApk}
                        loading={updatingApk}
                    >{androidStreamFileExists ? 'Update' : 'Get'}</StyledLoadingButton>
                    <StyledUploadLoadingButton
                        filename='gads-stream.apk'
                        allowedFileExtension='.apk'
                        tooltipText='Select and upload GADS-Android-stream apk'
                    ></StyledUploadLoadingButton>
                    <FileInfo
                        exists={androidStreamFileExists}
                        existsText='APK is available.'
                        notExistsText='APK not available.'
                    ></FileInfo>
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
                    <StyledUploadLoadingButton
                        component='label'
                        tooltipText='Select the WebDriverAgent ipa file from the file explorer'
                        allowedFileExtension='.ipa'
                        filename='WebDriverAgent.ipa'
                    ></StyledUploadLoadingButton>
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
                    <StyledButton>Download</StyledButton>
                    <StyledButton>Generate</StyledButton>
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
                <StyledUploadLoadingButton
                    filename='selenium.jar'
                    allowedFileExtension='.jar'
                    tooltipText='Select and upload Selenium standalone jar file'
                ></StyledUploadLoadingButton>
            </StyledBox>
        )
    }

    function IOSSupervisionBox() {
        return (
            <StyledBox>
                <h3>iOS supervision profile</h3>
                <p
                    style={{
                        marginTop: '5px',
                        textAlign: 'center',
                    }}
                >If you have supervised your iOS devices you can supply the supervision `.p12` file to allow providers to pair with devices without manual intervention</p>
                <StyledUploadLoadingButton
                    filename='supervision_profile.p12'
                    allowedFileExtension='.p12'
                    tooltipText='Select and upload iOS supervision profile'
                ></StyledUploadLoadingButton>
            </StyledBox>
        )
    }

    function SigningP12FileBox() {
        const canGenerate = signingCertificateFileExists && signingPemFileExists

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
                    alignItems={"center"}
                >
                    <StyledUploadLoadingButton
                        filename='signing_pkcs.p12'
                        allowedFileExtension='.p12'
                        tooltipText='Select and upload PKCS#12 (.p12) file'
                    ></StyledUploadLoadingButton>
                    <Divider width='100%'></Divider>
                    <StyledButton
                        disabled={!canGenerate}
                    >Generate</StyledButton>
                    <FileInfo
                        exists={signingPemFileExists}
                        existsText='Signing private key is available.'
                        notExistsText='No signing private key.'
                    ></FileInfo>
                    <FileInfo
                        exists={signingCertificateFileExists}
                        existsText='Signing certificate is available.'
                        notExistsText='No signing certificate.'
                    ></FileInfo>
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
                    <StyledUploadLoadingButton
                        filename='signing_key.pem'
                        allowedFileExtension='.pem'
                        tooltipText='Select and upload private signing key'
                    ></StyledUploadLoadingButton>
                    <StyledButton>Download</StyledButton>
                    <StyledLoadingButton
                        tooltipText={<div>Generate a new private key that can be used for creating CSR(certificate signing request) and re-signing of WebDriverAgent<br />!!! Note that this will replace your currently existing private key file</div>}
                        onClick={handleGeneratePrivateKeyClick}
                    >Generate</StyledLoadingButton>
                </Stack>
            </StyledBox>
        )
    }
}