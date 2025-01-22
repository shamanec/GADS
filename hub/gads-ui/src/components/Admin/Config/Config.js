import { Box, Button, Divider, Stack, Tooltip } from "@mui/material";
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
        border: '1px solid #ccc',
        borderRadius: '8px',
        padding: '10px',
        backgroundColor: '#f5f5f5',
        width: '300px',
        height: '400px',
        justifyContent: 'center',
        justifyItems: 'center'

    }))

    const StyledButton = styled(Button)(() => ({
        backgroundColor: '#2f3b26',
        color: '#f4e6cd',
        fontWeight: 'bold',
        boxShadow: 'none',
        height: '40px',
        '&:hover': {
            backgroundColor: '#2f3b26', // Prevent hover effect
            boxShadow: 'none',
        },
    }))

    // StyledButton.defaultProps = {
    //     variant: 'contained',
    // }

    return (
        <Stack>
            <SigningPrivateKeyBox
            ></SigningPrivateKeyBox>
            <CSRBox></CSRBox>
            <SigningP12FileBox></SigningP12FileBox>
            <WebDriverAgentBox></WebDriverAgentBox>
            <AndroidStreamBox></AndroidStreamBox>
            <SeleniumJarBox></SeleniumJarBox>
        </Stack>
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
                        console.log(updatingApk)
                    }, 1000)
                })
        }

        return (
            <StyledBox>
                <div>GADS Android stream</div>
                <Divider width='100%'></Divider>
                <Tooltip
                    arrow
                    placement='bottom'
                    title='Downloads the latest GADS Android stream apk from releases and updates it in the DB'
                >
                    <StyledButton
                        onClick={handleAndroidStreamUpdate}
                        disabled={updatingApk}
                    >{androidStreamFileExists ? 'Update' : 'Get'}</StyledButton>
                </Tooltip>
            </StyledBox>
        )
    }

    function CSRBox() {
        return (
            <StyledBox>
                <div>iOS CSR</div>
                <Divider></Divider>
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
                <div>Selenium jar</div>
                <Divider></Divider>
                <StyledButton>Upload</StyledButton>
            </StyledBox>
        )
    }

    function SigningP12FileBox() {
        return (
            <StyledBox>
                <div>iOS p12 signing file</div>
                <Divider></Divider>
                <StyledButton>Upload</StyledButton>
                <StyledButton>Download</StyledButton>
                <StyledButton>Generate</StyledButton>
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
                <div>iOS signing private key</div>
                <Divider></Divider>
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

            </StyledBox>
        )
    }

    function WebDriverAgentBox() {
        return (
            <StyledBox>
                <div>WebDriverAgent - real devices</div>
                <Divider></Divider>
                <StyledButton>Upload</StyledButton>
                <StyledButton>Re-sign</StyledButton>
            </StyledBox>
        )
    }
}