
import SmartphoneIcon from '@mui/icons-material/Smartphone';
import LanIcon from '@mui/icons-material/Lan';
import Tooltip from '@mui/material/Tooltip';
import { List, ListItem, ListItemIcon, ListItemText } from '@mui/material';
import Box from '@mui/material/Box';

export default function Info({ info }) {
    return (
        <Box
            sx={{ height: '100px', maxWidth: '200px', background: 'white', marginTop: '10px', borderRadius: '10px', marginLeft: '10px' }}
        >
            <Tooltip title='Configured devices' placement='left' leaveDelay={0}>
                <ListItem>
                    <ListItemIcon>

                        <SmartphoneIcon />


                    </ListItemIcon>
                    <ListItemText
                        primary={info.devices}
                    />
                </ListItem>
            </Tooltip>
            <Tooltip title='Provider address' placement='left'>
                <ListItem>
                    <ListItemIcon>
                        <LanIcon />
                    </ListItemIcon>
                    <ListItemText
                        primary={info.host_address}
                    />
                </ListItem>
            </Tooltip>
        </Box>
    )
}