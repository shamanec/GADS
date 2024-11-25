import { Stack } from '@mui/material'
import FileUploader from './FileUploader'
import { useEffect, useState } from 'react'
import { api } from '../../../services/api'

export default function FilesAdministration() {
    const [seleniumJarExists, setSeleniumJarExists] = useState(false)
    const [supervisionFileExists, setSupervisionFileExists] = useState(false)

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
                    }
                }
            })
            .catch(() => {
            })
    }

    useEffect(() => {
        handleGetFileData()
    }, [])

    return (
        <Stack
            style={{
                marginLeft: '20px',
                marginTop: '20px'
            }}
            direction='row'
            spacing={1}
        >
            <FileUploader
                title='Upload Selenium jar'
                description='If you want to connect provider Appium nodes to Selenium Grid instance you need to upload a valid Selenium jar. Version 4.13 is recommended.'
                allowedExtensions={['jar']}
                fileStatus={seleniumJarExists}
                fileName='selenium.jar'
                expectedExtension='.jar'
            />
            <FileUploader
                title='Upload supervision profile'
                description='Upload the supervision profile if you are using supervised iOS devices'
                uploadUrl='/admin/upload-supervision-profile'
                allowedExtensions={['p12']}
                fileStatus={supervisionFileExists}
                fileName='supervision.p12'
                expectedExtension='.p12'
            />
        </Stack>
    )
}