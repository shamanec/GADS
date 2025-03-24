import Tabs from '@mui/material/Tabs'
import Tab from '@mui/material/Tab'
import React, { useEffect, useState } from 'react'
import './Filters.css'
import { FiSearch } from 'react-icons/fi'
import { Box, FormControl, MenuItem, Tooltip, TextField} from '@mui/material'
import { api } from '../../services/api'

export function OSFilterTabs({ currentTabIndex, handleTabChange }) {
    return (
        <Tabs
            value={currentTabIndex}
            onChange={handleTabChange}
            TabIndicatorProps={{
                style: {
                    background: '#2f3b26',
                    height: '5px'
                }
            }}
            textColor='#f4e6cd'
            sx={{
                color: '#2f3b26',
                fontFamily: 'Verdana'
            }}
        >
            <Tab label='All' />
            <Tab label='Android' />
            <Tab label='iOS' />
        </Tabs>
    )
}

export function DeviceSearch({ keyUpFilterFunc }) {
    return (
        <div id='search-wrapper'>
            <div id='image-wrapper'>
                <FiSearch size={25} />
            </div>
            <input
                type='search'
                id='search-input'
                onInput={() => keyUpFilterFunc()}
                placeholder='Search devices'
                className='custom-placeholder'
                autoComplete='off'
            ></input>
        </div>
    )
}

export function WorkspaceSelector({ selectedWorkspace, setSelectedWorkspace }) {
    const [workspaces, setWorkspaces] = useState([]);

    useEffect(() => {
        fetchWorkspaces();
    }, []);

    const fetchWorkspaces = async () => {
        try {
            const response = await api.get('/workspaces?page=1&limit=10&search=');
            
            // Sort workspaces to ensure default workspace is always first
            const sortedWorkspaces = [...response.data.workspaces].sort((a, b) => {
                if (a.is_default) return -1;
                if (b.is_default) return 1;
                return 0;
            });
            
            setWorkspaces(sortedWorkspaces);
            
            // Find the default workspace
            const defaultWorkspace = sortedWorkspaces.find(ws => ws.is_default);
            if (defaultWorkspace) {
                setSelectedWorkspace(defaultWorkspace.id);
            } else if (sortedWorkspaces.length > 0) {
                setSelectedWorkspace(sortedWorkspaces[0].id);
            }
        } catch (error) {
            console.error('Failed to fetch workspaces:', error);
        }
    };

    return (
        <Box sx={{ minWidth: 200 }}>
            <Tooltip
                title='Devices Workspace'
                arrow
                placement='top'
            >
                <FormControl fullWidth size="small" style={{backgroundColor: '#878a91'}}>
                    <TextField
                        label='Workspace'
                        value={selectedWorkspace}
                        select
                        onChange={(e) => setSelectedWorkspace(e.target.value)}
                        sx={{
                            backgroundColor: '#f4e6cd',
                            borderRadius: '4px',
                            '& .MuiOutlinedInput-notchedOutline': {
                                borderColor: '#2f3b26',
                            },
                        }}
                        size='small'
                    >
                        {workspaces.map((workspace) => (
                            <MenuItem key={workspace.id} value={workspace.id}>
                                {workspace.name}
                            </MenuItem>
                        ))}
                    </TextField>
                </FormControl>
            </Tooltip>
        </Box>
    );
}