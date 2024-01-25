import axios from 'axios'

export const api = axios.create({
    baseURL: `http://${process.env.REACT_APP_PROVIDER_HOST}`
})