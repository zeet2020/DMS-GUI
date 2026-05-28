import React from 'react'
import {createRoot} from 'react-dom/client'
import '@mantine/core/styles.css'
import {createTheme, MantineProvider} from '@mantine/core'
import './style.css'
import App from './App'

const theme = createTheme({
    primaryColor: 'teal',
})

const container = document.getElementById('root')
const root = createRoot(container!)

root.render(
    <React.StrictMode>
        <MantineProvider theme={theme} defaultColorScheme="dark">
            <App/>
        </MantineProvider>
    </React.StrictMode>
)
