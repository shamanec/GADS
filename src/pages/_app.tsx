import type { AppProps } from 'next/app'

import { Header } from '@/components/Header'

import { SocketProvider } from '@/providers/socket'

import '@/styles/global.scss'

function MyApp({ Component, pageProps }: AppProps) {
  return (
    <SocketProvider>
      <Header />
      <Component {...pageProps} />
    </SocketProvider>
  )
}

export default MyApp