import { useEffect, useState } from 'react'
import { useSnackbar } from '../../../contexts/SnackBarContext'
import { api } from '../../../services/api'
import { Box, Button, CircularProgress, Typography } from '@mui/material'

export default function FileUploader({
    title = 'Upload File',
    description = '',
    allowedExtensions = [],
    onSuccess = () => { },
    onError = () => { },
    buttonLabel = 'Select and upload',
    fileStatus = false,
    fileName = '',
    expectedExtension = ''
}) {
    const [isUploading, setIsUploading] = useState(false)
    const [isUploaded, setIsUploaded] = useState(fileStatus)
    const { showSnackbar, hideSnackbar } = useSnackbar()
    const [inputKey, setInputKey] = useState(Date.now())

    const handleUpload = (e) => {
        hideSnackbar()
        if (e.target.files) {
            const targetFile = e.target.files[0]
            if (!targetFile) {
                return
            }

            const fileExtension = targetFile.name.split('.').pop().toLowerCase()

            // Validate file extension
            if (allowedExtensions.length > 0 && !allowedExtensions.includes(fileExtension)) {
                showCustomSnackbar({
                    message: `Invalid file extension. Allowed extensions: ${allowedExtensions.join(', ')}`,
                    timeout: 5000,
                })
                return
            }

            const form = new FormData()
            form.append('file', targetFile)
            form.append('fileName', fileName)
            form.append('extension', expectedExtension)

            setIsUploading(true)
            api.post('/admin/upload-file', form, {
                headers: {
                    'Content-Type': 'multipart/form-data',
                },
            })
                .then(() => {
                    showCustomSnackbar({
                        message: 'File uploaded!',
                        severity: 'success',
                        timeout: 2000,
                    })
                    setIsUploaded(true)
                    onSuccess()
                    setInputKey(Date.now())
                })
                .catch((error) => {
                    showCustomSnackbar({ message: 'File upload failed!' })
                    onError(error)
                    setInputKey(Date.now())
                })
                .finally(() => {
                    setTimeout(() => {
                        setIsUploading(false)
                    }, 1000)
                })
        }
    }

    const showCustomSnackbar = ({ message, severity = 'error', timeout = 3000 }) => {
        showSnackbar({
            message,
            severity,
            duration: timeout,
        })
    }

    useEffect(() => {
        setIsUploaded(fileStatus)
    }, [fileStatus])

    return (
        <Box
            style={{
                borderRadius: '10px',
                padding: '20px',
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                justifyContent: 'center',
                border: '1px solid #ddd',
                width: '280px',
                height: '280px',
                backgroundColor: '#9ba984'
            }}
        >
            <h3>{title}</h3>
            {description && (
                <p
                    style={{
                        marginTop: '5px',
                        textAlign: 'center',
                    }}
                >
                    {description}
                </p>
            )}
            <Button
                component='label'
                variant='contained'
                style={{
                    backgroundColor: '#2f3b26',
                    color: '#fff',
                    fontWeight: 'bold',
                    width: '80%'
                }}
                disabled={isUploading}
            >
                <input
                    key={inputKey}
                    type='file'
                    onChange={(event) => handleUpload(event)}
                    hidden
                />
                {isUploading ? (
                    <CircularProgress size={25} style={{ color: '#f4e6cd' }} />
                ) : (
                    buttonLabel
                )}
            </Button>
            <Typography variant='caption' style={{ marginTop: '10px', color: '#2f3b26' }}>
                {isUploaded ? 'File exists.' : 'No uploaded file.'}
            </Typography>
        </Box>
    )
}