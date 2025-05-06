import { api } from './api'

const axiosInterceptor = (logout) => {
    // intercept the response
    api.interceptors.response.use(
        (response) => {
            return response
        },
        (error) => {
            // Check if the error response exists and has status 401 (Unauthorized)
            if (error.response && error.response.status === 401) {
                localStorage.removeItem('accessToken')
                localStorage.removeItem('userRole')
                localStorage.removeItem('username')
                // Call the logout function safely
                if (typeof logout === 'function') {
                    logout()
                }
            }

            // Reject the error to propagate it to the request's catch block
            return Promise.reject(error)
        }
    )
}

export default axiosInterceptor