import { useState } from 'react'

import { api } from '../services/api'

export function useAdmin() {
    const [providers, setProviders] = useState([])

    const listProviders = async () => {
        try{
            const response = await api.get(`/admin/providers`)

            if(response.status === 200) {
                let providers = response.data ?? {}
                setProviders(providers)
                return providers
            }else {
                return {
                    success: false,
                    message: 'An unknown error has occurred.',
                    response: response.data
                }
            }
        }catch(error) {
            if(error.response) {
                if(error.response.status === 403) {
                    return {
                        success: false,
                        message: 'Incomplete request',
                        response: error.response
                    }
                }
            }

            return {
                success: false,
                message: 'An unknown error has occurred.',
                response: error.response
            }
        }
    }

    const registerProvider = async (os, host_address, nickaname, port, provider_android, provide_ios, wda_bundle_id, wda_repo_path, supervision_password, use_selenium_grid, selenium_grid) => {
        try{
            const response = await api.post(`/admin/providers/add`, {
                os,
                host_address,
                nickaname,
                port,
                provider_android,
                provide_ios,
                wda_bundle_id,
                wda_repo_path,
                supervision_password,
                use_selenium_grid,
                selenium_grid
            })

            if(response.status === 200) {
                let providers = response.data ?? {}
                setProviders(providers)
                return providers
            }else {
                return {
                    success: false,
                    message: 'An unknown error has occurred.',
                    response: response.data
                }
            }
        }catch(error) {
            if(error.response) {
                if(error.response.status === 403) {
                    return {
                        success: false,
                        message: 'Incomplete request',
                        response: error.response
                    }
                }
            }

            return {
                success: false,
                message: 'An unknown error has occurred.',
                response: error.response
            }
        }
    }

    const updateProvider = async (os, host_address, nickaname, port, provider_android, provide_ios, wda_bundle_id, wda_repo_path, supervision_password, use_selenium_grid, selenium_grid) => {
        try{
            const response = await api.post(`/admin/providers/update`, {
                os,
                host_address,
                nickaname,
                port,
                provider_android,
                provide_ios,
                wda_bundle_id,
                wda_repo_path,
                supervision_password,
                use_selenium_grid,
                selenium_grid
            })

            if(response.status === 200) {
                let providers = response.data ?? {}
                setProviders(providers)
                return providers
            }else {
                return {
                    success: false,
                    message: 'An unknown error has occurred.',
                    response: response.data
                }
            }
        }catch(error) {
            if(error.response) {
                if(error.response.status === 403) {
                    return {
                        success: false,
                        message: 'Incomplete request',
                        response: error.response
                    }
                }
            }

            return {
                success: false,
                message: 'An unknown error has occurred.',
                response: error.response
            }
        }
    }

    //TODO: Function to list users

    const registerUser = async (username, password, role, email) => {
        try{
            const response = await api.post(`/admin/user`, {
                username,
                password,
                role,
                email
            })

            if(response.status === 200) {
                let res = response.data ?? {}
                return res
            }else {
                return {
                    success: false,
                    message: 'An unknown error has occurred.',
                    response: response.data
                }
            }
        }catch(error) {
            if(error.response) {
                if(error.response.status === 403) {
                    return {
                        success: false,
                        message: 'Incomplete request',
                        response: error.response
                    }
                }
            }

            return {
                success: false,
                message: 'An unknown error has occurred.',
                response: error.response
            }
        }
    }

    return {
        providers,

        listProviders,
        registerProvider,
        updateProvider,

        registerUser
    }
}