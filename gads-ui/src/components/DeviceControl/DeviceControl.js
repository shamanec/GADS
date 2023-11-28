import { useParams } from 'react-router-dom';
import { useNavigate } from 'react-router-dom';
import StreamCanvas from './StreamCanvas'
import ActionsStack from './ActionsStack';
import { Stack } from '@mui/material';

export default function DeviceControl() {
    const { id } = useParams();
    const deviceData = JSON.parse(localStorage.getItem(id))

    return (
        <div>
            <div className='back-button-bar' style={{
                marginBottom: '10px',
                marginTop: '10px'
            }}>
                <BackButton />
            </div>
            <Stack direction={"row"} spacing={2} style={{ marginLeft: "20px" }}>
                <ActionsStack deviceData={deviceData}></ActionsStack>
                <StreamCanvas deviceData={deviceData}></StreamCanvas>
            </Stack>

        </div>
    )
}

function BackButton() {
    const navigate = useNavigate();

    const handleBackClick = () => {
        navigate('/devices');
    };

    return (
        <button onClick={handleBackClick}
            style={{
                borderRadius: '5px',
                backgroundColor: 'white',
                border: '1px solid #f3f3f5',
                padding: '5px'
            }}> &larr; Go back to devices</button>
    )
}