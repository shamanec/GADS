import { Box, Button, TextField, Grid, MenuItem, Stack, Typography } from '@mui/material';
import { useState, useEffect } from 'react';
import { api } from '../../../services/api';

// Import options from StreamSettings
const fpsOptions = [5, 10, 15, 20, 30, 45, 60];
const jpegQualityOptions = [25, 30, 35, 40, 45, 50, 55, 60, 65, 70, 75, 80, 85, 90];
const scalingFactorOptionsAndroid = [25, 50];
const scalingFactorOptionsiOS = [25, 30, 35, 40, 45, 50, 55, 60, 65, 70, 75, 80, 85, 90, 95, 100];

export default function GlobalSettings() {
    const [fps, setFps] = useState(15);
    const [jpegQuality, setJpegQuality] = useState(75);
    const [scalingFactorAndroid, setScalingFactorAndroid] = useState(50);
    const [scalingFactoriOS, setScalingFactoriOS] = useState(50);
    const [loading, setLoading] = useState(false);
    const [status, setStatus] = useState(null);

    useEffect(() => {
        // Fetch global stream settings on component mount
        api.get('/admin/global-settings')
            .then(response => {
                const settings = response.data;
                setFps(settings.target_fps);
                setJpegQuality(settings.jpeg_quality);
                setScalingFactorAndroid(settings.scaling_factor_android);
                setScalingFactoriOS(settings.scaling_factor_ios);
            })
            .catch(() => {
                setStatus('error');
            });
    }, []);

    const handleSave = () => {
        setLoading(true);
        const settings = { target_fps: fps, jpeg_quality: jpegQuality, scaling_factor_android: scalingFactorAndroid, scaling_factor_ios: scalingFactoriOS };

        api.post('/admin/global-settings', settings)
            .then(() => {
                setStatus('success');
            })
            .catch(() => {
                setStatus('error');
            })
            .finally(() => {
                setLoading(false);
                setTimeout(() => setStatus(null), 2000);
            });
    };

    return (
        <Stack direction='row' spacing={2} style={{ width: '100%', marginLeft: '10px', marginTop: '10px' }}>
            <Box style={{
                marginBottom: '10px',
                height: '80vh',
                overflowY: 'scroll',
                border: '2px solid black',
                borderRadius: '10px',
                boxShadow: 'inset 0 -10px 10px -10px #000000',
                scrollbarWidth: 'none',
                marginRight: '10px',
                width: '100%'
            }}>
                <Grid
                    container
                    spacing={2}
                    margin='10px'
                >
                    <Grid item>
                        <Box
                            id='stream-settings'
                            style={{
                                border: '1px solid black',
                                width: '400px',
                                padding: '20px',
                                minWidth: '400px',
                                maxWidth: '400px',
                                height: '830px',
                                borderRadius: '5px',
                                backgroundColor: '#9ba984'
                            }}
                        >
                            <Typography variant="h6" style={{ marginBottom: '10px' }}>Stream Settings</Typography>
                            <Stack spacing={2}>
                                <TextField
                                    label="Target FPS"
                                    select
                                    value={fps}
                                    onChange={(e) => setFps(e.target.value)}
                                    variant="outlined"
                                >
                                    {fpsOptions.map((option) => (
                                        <MenuItem key={option} value={option}>
                                            {option} FPS
                                        </MenuItem>
                                    ))}
                                </TextField>
                                <TextField
                                    label="JPEG Quality"
                                    select
                                    value={jpegQuality}
                                    onChange={(e) => setJpegQuality(e.target.value)}
                                    variant="outlined"
                                >
                                    {jpegQualityOptions.map((option) => (
                                        <MenuItem key={option} value={option}>
                                            {option}
                                        </MenuItem>
                                    ))}
                                </TextField>
                                <TextField
                                    label="Scaling Factor (Android)"
                                    select
                                    value={scalingFactorAndroid}
                                    onChange={(e) => setScalingFactorAndroid(e.target.value)}
                                    variant="outlined"
                                >
                                    {scalingFactorOptionsAndroid.map((option) => (
                                        <MenuItem key={option} value={option}>
                                            {option}%
                                        </MenuItem>
                                    ))}
                                </TextField>
                                <TextField
                                    label="Scaling Factor (iOS)"
                                    select
                                    value={scalingFactoriOS}
                                    onChange={(e) => setScalingFactoriOS(e.target.value)}
                                    variant="outlined"
                                >
                                    {scalingFactorOptionsiOS.map((option) => (
                                        <MenuItem key={option} value={option}>
                                            {option}%
                                        </MenuItem>
                                    ))}
                                </TextField>
                                <Button
                                    variant="contained"
                                    onClick={handleSave}
                                    disabled={loading}
                                    style={{
                                        backgroundColor: '#2f3b26',
                                        color: '#f4e6cd',
                                        fontWeight: 'bold'
                                    }}
                                >
                                    {loading ? 'Saving...' : 'Save Settings'}
                                </Button>
                                {status === 'success' && <Typography color="green">Settings saved successfully!</Typography>}
                                {status === 'error' && <Typography color="red">Failed to save settings!</Typography>}
                            </Stack>
                        </Box>
                    </Grid>
                </Grid>
            </Box>
        </Stack>
    );
} 