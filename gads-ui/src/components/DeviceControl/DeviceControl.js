import { useParams } from 'react-router-dom';
import { useNavigate } from 'react-router-dom';
import StreamCanvas from './StreamCanvas'
import ActionsStack from './ActionsStack';
import { Stack } from '@mui/material';
import { Button } from '@mui/material';
import TabularControl from './Tabs/TabularControl';

export default function DeviceControl() {
    const { id } = useParams();
    const navigate = useNavigate();
    const deviceData = JSON.parse(localStorage.getItem(id))

    const handleBackClick = () => {
        navigate('/devices');
    };

    return (
        <div>
            <div className='back-button-bar' style={{
                marginBottom: '10px',
                marginTop: '10px'
            }}>
                <Button
                    variant="contained"
                    onClick={handleBackClick}
                    style={{ marginLeft: "20px" }}
                >Back to devices</Button>
            </div>
            <Stack direction={"row"} spacing={2} style={{ marginLeft: "20px" }}>
                <ActionsStack deviceData={deviceData}></ActionsStack>
                <StreamCanvas deviceData={deviceData}></StreamCanvas>
                <TabularControl deviceData={deviceData}></TabularControl>
            </Stack>

        </div>
    )
}