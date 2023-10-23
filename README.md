## Before Getting Started

First, install the project dependencies:

```bash
npm install
# or
yarn add
```

This command will install all project dependencies.

## Project dependencies

| |About|
|---|---|
|[sass](https://www.npmjs.com/package/sass)|This package is a distribution of Dart Sass, compiled to pure JavaScript with no native code or external dependencies. It provides a command-line sass executable and a Node.js API.|
|[rethinkdb](https://www.npmjs.com/package/rethinkdb)|This package is the officially supported driver for querying a RethinkDB database from a JavaScript application.|
|[socket.io](https://www.npmjs.com/package/socket.io)|Socket.IO enables real-time bidirectional event-based communication.|  
|[socket.io-client](https://github.com/socketio/socket.io-client)|Realtime application framework (client)|  

## Getting Started

Start the development server:

```bash
npm run dev
# or
yarn dev
# or
pnpm dev
```

Open [http://localhost:3000](http://localhost:3000) with your browser to see the result.

You can start editing the page by modifying `pages/index.tsx`. The page auto-updates as you edit the file.

[API routes](https://nextjs.org/docs/api-routes/introduction) can be accessed on [http://localhost:3000/api/hello](http://localhost:3000/api/hello). This endpoint can be edited in `pages/api/hello.ts`.

The `pages/api` directory is mapped to `/api/*`. Files in this directory are treated as [API routes](https://nextjs.org/docs/api-routes/introduction) instead of React pages.

## Features
1. ❌ Provider logs for debugging  
2. ✅ Devices remote control(most of which is wrapper around Appium)
  * ✅ Android
    - ✅ `GADS-Android-stream` video stream - not as good as `minicap` but it is inhouse, can be used in case `minicap` fails for device;
    - basic device interaction:
        - ✅ Home;
        - ✅ Lock;
        - ✅ Unlock;
        - ❌ Type text (TODO);
        - ❌ Clear text (TODO).
    - basic remote control:
        - ✅ tap;
        - ✅ swipe.
    - ✅ basic Appium inspector (Just for android)
3. ❌ (Not tested yet) iOS remote control
  * iOS
    - `WDA mjpeg` video stream  
    - basic device interaction:
        - ❌ Home;
        - ❌ Lock;
        - ❌ Unlock;
        - ❌ Type text (TODO);
        - ❌ Clear text (TODO).
    - basic remote control:
        - ❌ tap;
        - ❌ swipe.
    - ❌ basic Appium inspector (Just for android).
4. ✅ Open the device in a new window (For automated tests);

## Environment variables

This project uses `.env` files to run the application, find the `.env.examples` to see how to setup your variables and create a new `.env.local` file.

