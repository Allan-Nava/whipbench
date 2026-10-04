FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends janus nodejs npm ca-certificates make g++ python3 && rm -rf /var/lib/apt/lists/*
WORKDIR /app
RUN npm init -y >/dev/null && npm pkg set type=module && npm install janus-whip-server@1.1.0 janus-whep-server@1.1.0
COPY videoroom.jcfg /etc/janus/janus.plugin.videoroom.jcfg
COPY streaming.jcfg /etc/janus/janus.plugin.streaming.jcfg
COPY server.js start.sh /app/
CMD ["sh", "/app/start.sh"]
