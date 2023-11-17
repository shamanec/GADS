import { useParams } from 'react-router-dom';
import { useNavigate } from 'react-router-dom';

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
            <div>{deviceData.udid}</div>
            <div>{deviceData.os}</div>
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