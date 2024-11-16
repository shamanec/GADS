import { Box, Button } from '@mui/material'
import AttachFileIcon from '@mui/icons-material/AttachFile'
import React, { useEffect, useState } from 'react'
import { api } from '../../../services/api'
import CircularProgress from '@mui/material/CircularProgress'
import { useSnackbar } from '../../../contexts/SnackBarContext'

export default function UploadSeleniumJar() {
    const [isUploading, setIsUploading] = useState(false)
    const { showSnackbar, hideSnackbar } = useSnackbar()

    function handleUpload(e) {
        hideSnackbar()
        if (e.target.files) {
            const targetFile = e.target.files[0]
            const fileExtension = targetFile.name.split('.').pop()

            // If the provided file does not have valid extension
            if (fileExtension !== 'jar') {
                showCustomSnackbar({ message: 'Invalid file extension, only `.jar` is allowed!', timeout: 5000 })
                return
            }

            const form = new FormData()
            form.append('file', targetFile)
            const url = `/admin/upload-selenium-jar`

            setIsUploading(true)
            api.post(url, form, {
                headers: {
                    'Content-Type': 'multipart/form-data'
                }
            })
                .then(() => {
                    showCustomSnackbar({ message: 'Selenium jar successfully uploaded!', severity: 'success', timeout: 3000 })
                    setIsUploading(false)
                })
                .catch(error => {
                    if (error.response) {
                        showCustomSnackbar('Selenium Grid jar upload failed!')
                        setIsUploading(false)
                    }
                    setIsUploading(false)
                })
        }
    }

    const showCustomSnackbar = ({ message, severity = 'error', timeout = 3000 }) => {
        showSnackbar({
            message: message,
            severity: severity,
            duration: timeout,
        })
    }

    useEffect(() => {
        return () => {
            hideSnackbar()
        }
    }, [])

    return (
        <Box
            id='upload-wrapper'
            style={{
                borderRadius: '10px',
                height: '240px',
                display: 'flex',
                flexDirection: 'column',
                alignContent: 'center',
                justifyContent: 'flex-start'
            }}
        >
            <h3>Upload Selenium jar</h3>
            <h5
                style={{
                    marginTop: '5px'
                }}
            >If you want to connect provider Appium nodes to Selenium Grid instance you need to upload a valid Selenium jar. Version 4.13 is recommended. File will be stored in Mongo and downloaded automatically by provider instances</h5>
            <Button
                component='label'
                variant='contained'
                startIcon={isUploading ? null : <AttachFileIcon />}
                style={{
                    backgroundColor: '#2f3b26',
                    color: '#9ba984',
                    fontWeight: 'bold'
                }}
            >
                <input
                    id='input-file'
                    type='file'
                    onChange={(event) => handleUpload(event)}
                />
                {isUploading ? (
                    <CircularProgress size={25} style={{ color: '#f4e6cd' }} />
                ) : (
                    'Select and upload'
                )}
            </Button>
        </Box>
    )
}