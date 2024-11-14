import { Alert, Box, Button } from "@mui/material";
import AttachFileIcon from "@mui/icons-material/AttachFile";
import React, { useContext, useState } from "react";
import { api } from "../../../services/api";
import { Auth } from "../../../contexts/Auth";
import CircularProgress from "@mui/material/CircularProgress";

export default function UploadSeleniumJar() {
    const [showAlert, setShowAlert] = useState(false)
    const [alertText, setAlertText] = useState()
    const [alertSeverity, setAlertSeverity] = useState()
    const [isUploading, setIsUploading] = useState(false)
    const { logout } = useContext(Auth)

    function handleUpload(e) {
        if (e.target.files) {
            const targetFile = e.target.files[0]
            const fileExtension = targetFile.name.split('.').pop();

            // If the provided file does not have valid extension
            if (fileExtension !== 'jar') {
                // Show an alert and disable the upload button
                setAlertSeverity('error')
                setAlertText('Invalid file extension, only `.jar` is allowed')
                setShowAlert(true)
                return
            }

            const form = new FormData();
            form.append('file', targetFile);
            const url = `/admin/upload-selenium-jar`

            setShowAlert(false)
            setIsUploading(true)
            api.post(url, form, {
                headers: {
                    'Content-Type': 'multipart/form-data'
                }
            })
                .then(response => {
                    setAlertSeverity('success')
                    setAlertText(response.data.message)
                    setShowAlert(true)
                    setIsUploading(false)
                })
                .catch(error => {
                    if (error.response) {
                        setAlertSeverity('error')
                        setAlertText(error.response.data.message)
                        setShowAlert(true)
                        setIsUploading(false)
                    }
                    setIsUploading(false)
                    setAlertSeverity('error')
                    setAlertText('Failed uploading Selenium jar file')
                    setShowAlert(true)
                })
                .finally(() => {
                    setTimeout(() => {
                        setShowAlert(false)
                    }, 5000)
                })
        }
    }

    return (
        <Box
            id='upload-wrapper'
            style={{
                borderRadius: '10px',
                height: '280px',
                display: 'flex',
                flexDirection: 'column',
                alignContent: 'center',
                justifyContent: 'flex-start'
            }}
        >
            <h3>Upload Selenium jar</h3>
            <h5
                style={{
                    marginTop: "5px"
                }}
            >If you want to connect provider Appium nodes to Selenium Grid instance you need to upload a valid Selenium jar. Version 4.13 is recommended. File will be stored in Mongo and downloaded automatically by provider instances</h5>
            <Button
                component='label'
                variant='contained'
                startIcon={isUploading ? null : <AttachFileIcon />}
                style={{
                    backgroundColor: "#2f3b26",
                    color: "#9ba984",
                    fontWeight: "bold"
                }}
            >
                <input
                    id='input-file'
                    type="file"
                    onChange={(event) => handleUpload(event)}
                />
                {isUploading ? (
                    <CircularProgress size={25} style={{ color: '#f4e6cd' }} />
                ) : (
                    'Select and upload'
                )}
            </Button>
            {showAlert && <Alert size='small' severity={alertSeverity} style={{ marginTop: '5px', padding: '2px 4px' }}>{alertText}</Alert>}
        </Box>
    )
}