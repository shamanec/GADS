
import SmartphoneIcon from '@mui/icons-material/Smartphone';
import LanIcon from '@mui/icons-material/Lan';
import Tooltip from '@mui/material/Tooltip';
import HomeIcon from '@mui/icons-material/Home';
import DesktopWindowsIcon from '@mui/icons-material/DesktopWindows';
import { ListItem, ListItemIcon, ListItemText } from '@mui/material';
import Box from '@mui/material/Box';
import { Button } from '@mui/material';

export default function Info({ info, handleSelectProvider }) {
    const name = info.nickname
    return (
        <Box
            sx={{
                maxWidth: '200px',
                background: 'white',
                borderRadius: '10px',
                marginLeft: '10px',
                alignItems: 'center',
                display: 'flex',
                flexDirection: 'column'
            }}
        >
            <ListItem>
                <Tooltip title='Nickname' placement='bottom' leaveDelay={0}>
                    <ListItemIcon>
                        <HomeIcon />
                    </ListItemIcon>
                </Tooltip>
                <ListItemText
                    primary={info.nickname}
                    style={{ wordWrap: 'break-word' }}
                />
            </ListItem>
            <ListItem>
                <Tooltip title='Provider address' placement='bottom'>
                    <ListItemIcon>
                        <LanIcon />
                    </ListItemIcon>
                </Tooltip>
                <ListItemText
                    primary={info.host_address}
                    style={{ wordWrap: 'break-word' }}
                />
            </ListItem>
            <ListItem>
                <Tooltip title='Port' placement='bottom'>
                    <ListItemIcon>
                        <LanIcon />
                    </ListItemIcon>
                </Tooltip>
                <ListItemText
                    primary={info.port}
                    style={{ wordWrap: 'break-word' }}
                />
            </ListItem>
            <ListItem>
                <Tooltip title='OS' placement='bottom'>
                    <ListItemIcon>
                        <DesktopWindowsIcon />
                    </ListItemIcon>
                </Tooltip>
                <ListItemText
                    primary={info.os}
                />
            </ListItem>
            <Button
                variant='contained'
                style={{ width: '80%', marginBottom: '5px' }}
                onClick={() => handleSelectProvider(name)}
            >Select</Button>
        </Box>
    )
}