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
                    background: "#0c111e",
                    height: "5px"
                }
            }}
            textColor='#0c111e'
            sx={{
                color: "#0c111e",
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
                <FiSearch size={25}/>
            </div>
            <input
                type="search"
                id="search-input"
                onInput={() => keyUpFilterFunc()}
                placeholder="Search devices"
                className="custom-placeholder"
            ></input>
        </div>
    )
}