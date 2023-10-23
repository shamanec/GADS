FROM node:lts as dependencies
WORKDIR /device-farm-frontend
COPY package.json package-lock.json ./
RUN npm install --frozen-lockfile

FROM node:lts as builder
WORKDIR /device-farm-frontend
COPY . .
COPY --from=dependencies /device-farm-frontend/node_modules ./node_modules
RUN npm run build

FROM node:lts as runner
WORKDIR /device-farm-frontend
# If you are using a custom next.config.js file, uncomment this line.
# COPY --from=builder /device-farm-frontend/next.config.js ./
COPY --from=builder /device-farm-frontend/public ./public
COPY --from=builder /device-farm-frontend/.next ./.next
COPY --from=builder /device-farm-frontend/node_modules ./node_modules
COPY --from=builder /device-farm-frontend/package.json ./package.json

EXPOSE 3000
CMD ["npm", "start"]
