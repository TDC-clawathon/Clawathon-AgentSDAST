#!/usr/bin/env python3
"""AgentBase health contract: GET /health must return HTTP 200."""
import os
import subprocess
from http.server import BaseHTTPRequestHandler, HTTPServer

PORT = int(os.environ.get("AGENTBASE_HEALTH_PORT", "8080"))
CHECK_CMD = os.environ.get("HEALTH_CHECK_CMD", "")


class HealthHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path.rstrip("/") != "/health":
            self.send_response(404)
            self.end_headers()
            return

        if not CHECK_CMD:
            self.send_response(503)
            self.end_headers()
            return

        try:
            subprocess.run(
                CHECK_CMD,
                shell=True,
                check=True,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                timeout=5,
            )
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(b'{"status":"ok"}')
        except (subprocess.CalledProcessError, subprocess.TimeoutExpired, OSError):
            self.send_response(503)
            self.end_headers()

    def log_message(self, _format, *_args):
        return


def main():
    server = HTTPServer(("0.0.0.0", PORT), HealthHandler)
    print(f"AgentBase health proxy listening on :{PORT}", flush=True)
    server.serve_forever()


if __name__ == "__main__":
    main()
