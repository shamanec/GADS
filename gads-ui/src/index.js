import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter as Router } from 'react-router-dom'

import Gads from './Gads'

import { AuthProvider } from './contexts/Auth'

import './styles/global.scss'

const root = ReactDOM.createRoot(document.getElementById('root'))
root.render(
  <AuthProvider>
    <Router>
      <Gads />
    </Router>
  </AuthProvider>
)
