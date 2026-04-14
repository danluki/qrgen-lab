FROM oven/bun:1.1.38
WORKDIR /app
COPY package.json ./
RUN bun install
COPY . .
RUN bun run build
EXPOSE 4173
CMD ["bunx", "vite", "preview", "--host", "0.0.0.0", "--port", "4173"]
