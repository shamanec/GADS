import Tabs from '@mui/material/Tabs';
import Tab from '@mui/material/Tab';
import React, { useEffect, useState } from 'react'
import './Filters.css'
import { FiSearch } from "react-icons/fi";

export function OSFilterTabs({ currentTabIndex, handleTabChange }) {
    return (
        <Tabs
            value={currentTabIndex}
            onChange={handleTabChange}
            TabIndicatorProps={{
                style: {
                    background: "#282c34",
                    height: "5px"
                }
            }}
            textColor='#282c34'
            sx={{
                color: "#282c34",
                fontFamily: "Verdana"
            }}
        >
            <Tab label="All" />
            <Tab label="Android" />
            <Tab label="iOS" />
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
                type="search"
                id="search-input"
                onKeyUp={() => keyUpFilterFunc()}
                placeholder="Search devices"
            ></input>
        </div>
    )
}