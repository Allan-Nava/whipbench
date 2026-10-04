janus -F /etc/janus -o -b &
sleep 3
exec node /app/server.js
