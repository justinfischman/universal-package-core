import { createServer } from "node:http";
import type { IncomingMessage, ServerResponse } from "node:http";

// Usage: node main.ts [port]. It listens on 127.0.0.1; set APP_HOST to listen
// elsewhere (the Dockerfile does).
const port: number = Number(process.argv[2] ?? "8000");
const host: string = process.env["APP_HOST"] ?? "127.0.0.1";

const server = createServer((_request: IncomingMessage, response: ServerResponse): void => {
  response.writeHead(200, { "Content-Type": "text/plain; charset=utf-8" });
  response.end("ok\n");
});

server.listen(port, host, (): void => {
  console.log(`listening on http://${host}:${port}`);
});
