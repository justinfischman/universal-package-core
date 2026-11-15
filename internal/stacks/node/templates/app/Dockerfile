FROM node:24-slim AS build
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm install
COPY tsconfig.json ./
COPY src ./src
RUN npm run build

FROM node:24-slim
WORKDIR /app
COPY package.json ./
COPY --from=build /app/dist ./dist
ENV APP_HOST=0.0.0.0
EXPOSE 8000
CMD ["node", "dist/main.js", "8000"]
